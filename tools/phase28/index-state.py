#!/usr/bin/env python3
"""Observe a Linux benchmark index or deliberately stop its Gemma child."""
from datetime import UTC, datetime
from pathlib import Path
import argparse
import json
import os
import signal
import sqlite3
import time


def progress(section):
    manifest = section / '.pbrain/index/manifest'
    for _ in range(5):
        try:
            stamp = manifest.stat()
            meta = json.loads(manifest.read_text())
            if stamp.st_ino != manifest.stat().st_ino:
                continue
            path = manifest.parent / meta['db']
            with sqlite3.connect(path.as_uri() + '?mode=ro&immutable=1', uri=True) as index:
                items = index.execute('SELECT count(*) FROM item').fetchone()[0]
                passages = index.execute('SELECT count(*) FROM passage').fetchone()[0]
            return {'published_at': stamp.st_mtime, 'items': items, 'passages': passages,
                    'database': str(path), 'manifest': meta}
        except (FileNotFoundError, sqlite3.OperationalError):
            continue
    return None


def process_memory(proc):
    return {line.partition(':')[0]: int(line.split()[1])
            for line in (proc / 'status').read_text().splitlines()
            if line.startswith(('VmRSS:', 'VmHWM:'))}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=('sample', 'pause'))
    parser.add_argument('work', type=Path)
    parser.add_argument('model', choices=('bge', 'gemma'))
    parser.add_argument('pid', type=int)
    args = parser.parse_args()
    work = args.work.resolve(strict=True)
    corpus = work / f'{args.model}-corpus'
    checkout = corpus / 'personal'
    brain = checkout / 'docs/brain'
    cache = corpus / 'personal-model-cache'
    proc = Path(f'/proc/{args.pid}')
    expected = [str(work / 'pbrain-baseline'), '--personal', 'index', '--json']
    actual = (proc / 'cmdline').read_bytes().rstrip(b'\0').decode().split('\0')
    if actual != expected or (proc / 'cwd').resolve(strict=True) != checkout:
        raise SystemExit('PID does not name the requested benchmark index process')
    if args.action == 'sample':
        while True:
            try:
                memory = process_memory(proc)
            except FileNotFoundError:
                break
            if not memory:
                break
            print(json.dumps({'observed_at': datetime.now(UTC).isoformat(),
                              'rss_kib': memory['VmRSS'], 'peak_rss_kib': memory['VmHWM'],
                              'shared': progress(brain), 'personal': progress(brain / 'personal')}), flush=True)
            time.sleep(60)
        return

    output = work / 'gemma-paused.json'
    if args.model != 'gemma' or output.exists() or not cache.is_dir():
        raise SystemExit('pause requires a first Gemma segment and its retained cache')
    if progress(brain) is None or progress(brain / 'personal') is not None:
        raise SystemExit('pause requires a published shared checkpoint before the personal build')
    ticks = int((proc / 'stat').read_text().rpartition(') ')[2].split()[19])
    peak = process_memory(proc)['VmHWM']
    active = time.clock_gettime(time.CLOCK_BOOTTIME) - ticks / os.sysconf('SC_CLK_TCK')
    os.kill(args.pid, signal.SIGTERM)
    # Let the child exit before reading the final immutable generation.
    while True:
        try:
            if not (proc / 'cmdline').read_bytes():
                break
        except FileNotFoundError:
            break
        time.sleep(.05)
    saved = progress(brain)
    if saved is None:
        raise SystemExit('no checkpoint after stopping the child')
    record = {'paused_at_utc': datetime.now(UTC).isoformat(), 'pid': args.pid,
              'initial_index_wall_seconds': active, 'peak_rss_kib': peak,
              'brain': str(brain), 'cache': str(cache),
              'checkpoint_items': saved['items'], 'checkpoint_passages': saved['passages'],
              'checkpoint_database': saved['database'], 'last_seen_manifest': saved['manifest'],
              'cache_vectors': sum(1 for p in (cache / 'vectors').rglob('*') if p.is_file())}
    output.write_text(json.dumps(record, indent=2) + '\n')
    print(output)


if __name__ == '__main__':
    main()

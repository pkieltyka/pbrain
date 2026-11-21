#!/usr/bin/env python3
"""Observe a Linux benchmark index or deliberately stop its Gemma child."""
from datetime import UTC, datetime
from pathlib import Path
import argparse
import json
import os
import select
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


def observe(args, pidfd):
    work = args.work.resolve(strict=True)
    corpus = work / f'{args.model}-corpus'
    checkout = corpus / 'personal'
    brain = checkout / 'docs/brain'
    cache = corpus / 'personal-model-cache'
    proc = Path(f'/proc/{args.pid}')
    exited = select.poll()
    exited.register(pidfd, select.POLLIN)
    expected = [os.fsencode(work / 'pbrain-baseline'), b'--personal', b'index', b'--json']
    invalid_pid = 'PID does not name a live benchmark index process for this fixture'
    try:
        actual = (proc / 'cmdline').read_bytes().rstrip(b'\0').split(b'\0')
        cwd = (proc / 'cwd').resolve(strict=True)
    except (FileNotFoundError, ProcessLookupError):
        raise SystemExit(invalid_pid) from None
    # If the PID was recycled during the /proc reads, the original pidfd is readable.
    if exited.poll(0) or actual != expected or cwd != checkout:
        raise SystemExit(invalid_pid)
    if args.action == 'sample':
        while True:
            try:
                memory = process_memory(proc)
            except (FileNotFoundError, ProcessLookupError):
                break
            if not memory or exited.poll(0):
                break
            print(json.dumps({'observed_at': datetime.now(UTC).isoformat(),
                              'rss_kib': memory['VmRSS'], 'peak_rss_kib': memory['VmHWM'],
                              'shared': progress(brain), 'personal': progress(brain / 'personal')}), flush=True)
            if exited.poll(60_000):
                break
        return

    output = work / 'gemma-paused.json'
    if args.model != 'gemma' or output.exists() or not cache.is_dir():
        raise SystemExit('pause requires a first Gemma segment and its retained cache')
    if progress(brain) is None or progress(brain / 'personal') is not None:
        raise SystemExit('pause requires a published shared checkpoint before the personal build')
    already_exited = 'benchmark index process exited before it could be paused'
    try:
        ticks = int((proc / 'stat').read_text().rpartition(') ')[2].split()[19])
        peak = process_memory(proc).get('VmHWM')
    except (FileNotFoundError, ProcessLookupError):
        raise SystemExit(already_exited) from None
    if peak is None or exited.poll(0):
        raise SystemExit(already_exited)
    active = time.clock_gettime(time.CLOCK_BOOTTIME) - ticks / os.sysconf('SC_CLK_TCK')
    try:
        signal.pidfd_send_signal(pidfd, signal.SIGTERM)
    except ProcessLookupError:
        raise SystemExit(already_exited) from None
    # Let the child exit before reading the final immutable generation.
    exited.poll()
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


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=('sample', 'pause'))
    parser.add_argument('work', type=Path)
    parser.add_argument('model', choices=('bge', 'gemma'))
    parser.add_argument('pid', type=int)
    args = parser.parse_args()
    if args.pid < 1:
        parser.error('pid must be positive')
    try:
        pidfd = os.pidfd_open(args.pid)
    except OSError as err:
        raise SystemExit(f'cannot open pidfd for {args.pid}: {err.strerror}') from None
    try:
        observe(args, pidfd)
    finally:
        os.close(pidfd)


if __name__ == '__main__':
    main()

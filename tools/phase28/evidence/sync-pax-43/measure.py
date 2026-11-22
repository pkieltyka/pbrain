import csv
import hashlib
import json
import os
from pathlib import Path
import select
import shutil
import sqlite3
import statistics
import subprocess
import time


root = Path(__file__).resolve().parent
source = Path("/var/tmp/pbrain-phase28-bge-34.rb61v3/corpus")
model_dirs = {
    "bge": Path("/var/tmp/pbrain-phase28-bge-pax-fresh.rCsjGe/bge-model"),
    "gemma": Path("/var/tmp/pbrain-phase28-gemma-22.5UFzx5/gemma-pack"),
}
cases = [
    ("bge-16-1", "bge", 16), ("bge-32-1", "bge", 32),
    ("bge-32-2", "bge", 32), ("bge-16-2", "bge", 16),
    ("bge-16-3", "bge", 16), ("bge-32-3", "bge", 32),
    ("gemma-cuda", "gemma", 4),
]
page_kib = os.sysconf("SC_PAGE_SIZE") // 1024
clock_ticks = os.sysconf("SC_CLK_TCK")


def append(path, record):
    with path.open("a") as handle:
        print(json.dumps(record, sort_keys=True), file=handle)


def utc():
    return time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())


def gpu(*args):
    return subprocess.check_output(["nvidia-smi", "-i", "0", *args], text=True).strip()


def cpu():
    fields = list(map(int, Path("/proc/stat").read_text().splitlines()[0].split()[1:9]))
    return sum(fields), sum(fields) - fields[3] - fields[4]


def fixture_digest(brain):
    digest = hashlib.sha256()
    shared = personal = total = 0
    for note in sorted(brain.rglob("*.md")):
        rel = note.relative_to(brain)
        content = note.read_bytes()
        digest.update(os.fsencode(str(rel)) + b"\0" + hashlib.sha256(content).digest())
        total += len(content)
        if rel.parts[0] == "personal":
            personal += 1
        else:
            shared += 1
    assert (shared, personal, total) == (4500, 500, 50_000_000), (shared, personal, total)
    return digest.hexdigest()


def ignore_indexes(dir_path, names):
    if Path(dir_path).name == ".pbrain":
        return {"index", "lock"}.intersection(names)
    return set()


def prepare(case_dir, model, expected_digest):
    corpus = case_dir / "corpus"
    shutil.copytree(source, corpus, ignore=ignore_indexes)
    brain = corpus / "docs/brain"
    digest = fixture_digest(brain)
    assert digest == expected_digest
    name = "bge-small-en-v1.5" if model == "bge" else "embeddinggemma-2"
    (brain / "pbrain.toml").write_text(f'[semantic]\nmodel = "{name}"\n')
    (brain / "personal.toml").write_text(f'model_path = "{model_dirs[model]}"\n')
    assert not list(brain.glob("**/.pbrain/index"))
    # Prime the verified model's files before timing. Index and vector caches start empty.
    for path in sorted(model_dirs[model].iterdir()):
        if path.is_file():
            with path.open("rb") as handle:
                while handle.read(8 * 1024 * 1024):
                    pass
    return corpus


def await_quiet(label):
    good = 0
    while good < 2:
        total0, busy0 = cpu()
        time.sleep(10)
        total1, busy1 = cpu()
        occupancy = (busy1 - busy0) / (total1 - total0)
        compute = gpu("--query-compute-apps=pid,used_memory", "--format=csv,noheader")
        quiet = occupancy <= 0.04 and not compute
        good = good + 1 if quiet else 0
        append(root / "preflight.jsonl", {
            "at": utc(), "case": label, "host_busy_pct": 100 * occupancy,
            "loadavg": Path("/proc/loadavg").read_text().strip(),
            "quiet_samples": good, "nvidia_compute": compute,
        })


def timed(case_dir, cmd, env, corpus, with_gpu):
    sampler = None
    if with_gpu:
        with (case_dir / "gpu.csv").open("w") as samples:
            sampler = subprocess.Popen(
                ["stdbuf", "-oL", "nvidia-smi", "-i", "0", "--query-gpu=timestamp,memory.used,utilization.gpu",
                 "--format=csv,noheader,nounits", "-lms", "250"], stdout=samples,
            )
    start_cpu = cpu()
    start = time.monotonic()
    max_sample_rss = 0
    try:
        with (case_dir / "sync.json").open("w") as out, (case_dir / "sync.stderr").open("w") as err:
            proc = subprocess.Popen(cmd, env=env, cwd=corpus, stdout=out, stderr=err)
            with (case_dir / "pid").open("w") as handle:
                print(proc.pid, file=handle)
            pidfd = os.pidfd_open(proc.pid)
            poll = select.poll()
            poll.register(pidfd, select.POLLIN)
            try:
                while not poll.poll(10000):
                    try:
                        # The timed PID includes all in-process Go threads (cutting, compute and writes).
                        tail = Path(f"/proc/{proc.pid}/stat").read_text().rsplit(")", 1)[1].split()
                        cpu_ticks = int(tail[11]) + int(tail[12])
                        rss = int(tail[21]) * page_kib
                    except FileNotFoundError:
                        break
                    total, busy = cpu()
                    elapsed = time.monotonic() - start
                    max_sample_rss = max(max_sample_rss, rss)
                    append(case_dir / "samples.jsonl", {
                        "at": utc(), "elapsed_s": elapsed, "pid": proc.pid,
                        "rss_kib": rss, "worker_cpu_s": cpu_ticks / clock_ticks,
                        "host_busy_cpu_s": (busy - start_cpu[1]) / clock_ticks,
                        "host_total_cpu_s": (total - start_cpu[0]) / clock_ticks,
                    })
            finally:
                os.close(pidfd)
            _, status, usage = os.wait4(proc.pid, 0)
            proc.returncode = os.waitstatus_to_exitcode(status)
    finally:
        if sampler is not None:
            sampler.terminate()
            sampler.wait(timeout=10)
    elapsed = time.monotonic() - start
    host_cpu_s = (cpu()[1] - start_cpu[1]) / clock_ticks
    child_cpu_s = usage.ru_utime + usage.ru_stime
    return {
        "wall_s": elapsed, "exit": proc.returncode, "host_peak_rss_kib": usage.ru_maxrss,
        "sample_peak_rss_kib": max_sample_rss, "process_cpu_s": child_cpu_s,
        "mean_other_logical_cpus": max(host_cpu_s - child_cpu_s, 0) / elapsed,
    }


def run(label, model, threads, expected_digest, uuid):
    case_dir = root / label
    case_dir.mkdir()
    corpus = prepare(case_dir, model, expected_digest)
    env = {**os.environ, "GOMAXPROCS": str(threads), "PBRAIN_CACHE": str(case_dir / "cache"),
           "GIT_CONFIG_GLOBAL": os.devnull, "GIT_CONFIG_NOSYSTEM": "1", "LC_ALL": "C", "TZ": "UTC"}
    for key in ("PBRAIN_DEVICE", "PBRAIN_GPU_MEMORY", "PBRAIN_BRAIN", "PBRAIN_DIR", "PBRAIN_ALLOW_GEMMA2",
                "PBRAIN_PROFILE_DIR", "GODEBUG", "GOEXPERIMENT"):
        env.pop(key, None)
    args = ["--personal", "sync", "--json"]
    with_gpu = model == "gemma"
    binary = root / ("pbrain-cuda" if with_gpu else "pbrain-cpu")
    if with_gpu:
        env.update(PBRAIN_ALLOW_GEMMA2="1", CUDA_VISIBLE_DEVICES=uuid,
                   LD_LIBRARY_PATH="/run/opengl-driver/lib")
        args += ["--device", "cuda:0"]
    else:
        env["CUDA_VISIBLE_DEVICES"] = ""
    affinity = f"0-{threads - 1}"
    cmd = ["taskset", "-c", affinity, str(binary), *args]
    print(f"{utc()} {label} waiting for idle host", flush=True)
    await_quiet(label)
    append(root / "events.jsonl", {"at": utc(), "case": label, "event": "start",
           "fixture_sha256": expected_digest, "command": cmd, "GOMAXPROCS": threads})
    print(f"{utc()} {label} started with {affinity}", flush=True)
    stat = timed(case_dir, cmd, env, corpus, with_gpu)
    result = {"at": utc(), "case": label, "model": model, "threads": threads, "cpu_affinity": affinity, **stat}
    if stat["exit"] != 0:
        append(root / "summary.jsonl", result)
        raise RuntimeError(f"{label} failed: {stat['exit']}. See {case_dir / 'sync.stderr'}")
    sections = json.loads((case_dir / "sync.json").read_text())["sections"]
    assert [r["items"] for r in sections] == [4500, 500], sections
    counts = [r["passages_embedded"] + r["passages_reused"] for r in sections]
    assert counts == [43642, 4869], sections
    device = "cuda:0" if with_gpu else "cpu"
    assert all(r["device"] == device for r in sections), sections
    result["passages"] = sum(counts)
    result["passages_embedded"] = sum(r["passages_embedded"] for r in sections)
    result["passages_reused"] = sum(r["passages_reused"] for r in sections)
    result["passages_per_s"] = result["passages_embedded"] / stat["wall_s"]
    if with_gpu:
        with (case_dir / "gpu.csv").open() as handle:
            gpu_samples = [(int(row[1]), int(row[2])) for row in csv.reader(handle)]
        result["device_peak_mib"] = max(vram for vram, util in gpu_samples)
        result["device_samples"] = len(gpu_samples)
        result["device_sample_median_util_pct"] = statistics.median(util for vram, util in gpu_samples)
    append(root / "summary.jsonl", result)
    print(json.dumps(result), flush=True)
    return result


def check_comparisons():
    baseline = root / "bge-16-1/corpus"
    with (root / "bge-database-comparisons.jsonl").open("w") as out:
        for label, model, _ in cases[1:]:
            if model == "bge":
                print(json.dumps({"case": label}), file=out, flush=True)
                subprocess.run(["python3", str(root / "compare.py"), str(baseline), str(root / label / "corpus")],
                               stdout=out, check=True)
    for section in ("shared", "personal"):
        brain = root / "gemma-cuda/corpus/docs/brain"
        if section == "personal":
            brain /= "personal"
        index = brain / ".pbrain/index"
        manifest = json.loads((index / "manifest").read_text())
        db = index / manifest["db"]
        with sqlite3.connect(db.as_uri() + "?mode=ro&immutable=1", uri=True) as conn:
            assert conn.execute("PRAGMA quick_check").fetchall() == [("ok",)]
        with db.open("rb") as handle:
            sha = hashlib.file_digest(handle, "sha256").hexdigest()
        append(root / "gemma-databases.jsonl", {"section": section, "manifest": manifest, "sha256": sha, "quick_check": "ok"})


if __name__ == "__main__":
    uuid = gpu("--query-gpu=uuid", "--format=csv,noheader")
    fixture_sha = fixture_digest(source / "docs/brain")
    append(root / "events.jsonl", {"at": utc(), "event": "suite-start", "source": str(source),
           "fixture_sha256": fixture_sha, "nvidia_uuid": uuid})
    for name, model, cpus in cases:
        run(name, model, cpus, fixture_sha, uuid)
    check_comparisons()
    (root / "complete").write_text(utc() + "\n")

"""Replace runs with sustained unrelated load and summarize the matched trials."""
import json
import statistics
import subprocess
import time

from measure import append, fixture_digest, gpu, root, run, source, utc


def lines(path):
    return [json.loads(line) for line in path.read_text().splitlines()]


def outside(case):
    samples = lines(root / case / "samples.jsonl")
    intervals = []
    sustained = maximum_streak = 0
    for before, after in zip(samples, samples[1:]):
        background = after["host_busy_cpu_s"] - before["host_busy_cpu_s"]
        child = after["worker_cpu_s"] - before["worker_cpu_s"]
        other = (background - child) / (after["elapsed_s"] - before["elapsed_s"])
        intervals.append(other)
        sustained = sustained + 1 if other >= 4 else 0
        maximum_streak = max(maximum_streak, sustained)
    return {
        "case": case,
        "peak_other_logical_cpus": max(intervals),
        "max_consecutive_intervals_with_four_or_more_other_logical_cpus": maximum_streak,
        "excluded_for_background_load": maximum_streak >= 3,
    }


def clean_trials():
    included = {16: [], 32: []}
    audit = []
    for trial in lines(root / "summary.jsonl"):
        assert trial["exit"] == 0, trial
        if trial["model"] == "bge":
            load = outside(trial["case"])
            audit.append(load)
            if not load["excluded_for_background_load"]:
                included[trial["threads"]].append(trial)
    return included, audit


def metric(trials, field):
    values = [row[field] for row in trials]
    return {"median": statistics.median(values), "min": min(values), "max": max(values)}


if __name__ == "__main__":
    while not (root / "complete").exists():
        unit = subprocess.run(
            ["systemctl", "--user", "is-active", "--quiet", "pbrain-sync-43.service"],
            check=False,
        )
        assert unit.returncode == 0 or (root / "complete").exists(), "original #43 suite stopped without completion"
        time.sleep(30)
    original = root / "bge-16-1/corpus"
    groups, checks = clean_trials()
    uuid = gpu("--query-gpu=uuid", "--format=csv,noheader")
    digest = fixture_digest(source / "docs/brain")
    attempts = 0
    for threads in (16, 32):
        while len(groups[threads]) < 3:
            attempts += 1
            label = f"bge-{threads}-repeat-{attempts}"
            row = run(label, "bge", threads, digest, uuid)
            with (root / f"{label}.comparison.jsonl").open("x") as out:
                subprocess.run(["python3", str(root / "compare.py"), str(original), str(root / label / "corpus")],
                               check=True, stdout=out)
            groups, checks = clean_trials()
    for check in checks:
        append(root / "load-audit.jsonl", check)
    summary = {
        "bge": {
            workers: {
                "trials": [t["case"] for t in group],
                "wall_s": metric(group, "wall_s"),
                "passages_per_s": metric(group, "passages_per_s"),
                "host_peak_rss_kib": metric(group, "host_peak_rss_kib"),
            }
            for workers, group in groups.items()
        },
        "gemma": next(t for t in lines(root / "summary.jsonl") if t["case"] == "gemma-cuda"),
    }
    append(root / "matched-summary.jsonl", summary)
    (root / "matched-complete").write_text(utc() + "\n")

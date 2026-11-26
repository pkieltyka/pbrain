"""Verify the retained full batch record and its historical comparison."""

import json
import statistics
import sys
from pathlib import Path


def lines(file):
    with file.open() as rows:
        return list(map(json.loads, rows))


def summarize(root):
    evidence = root / "evidence"
    (batch,) = lines(evidence / "full.jsonl")
    previous = json.loads((evidence / "historical-singleton.json").read_text())
    assert batch["label"] == "full-c" and batch["batch"] and batch["exit"] == 0
    assert previous["label"] == "sync-q8" and previous["exit"] == 0
    assert min(batch["idle_pct"]) >= 95
    for record in (batch, previous):
        sections = record["sections"]
        assert [s["items"] for s in sections] == [4500, 500]
        assert [s["passages_embedded"] for s in sections] == [43642, 4869]
        assert all(s["passages_reused"] == 0 for s in sections)
        assert abs(record["passages_s"] - 48511 / record["wall_s"]) < 1e-10
    assert batch["positions"] == 24475852
    assert batch["forwards"] == batch["spans"]["forward"]["calls"]
    assert sum(int(k) * v for k, v in batch["shape_counts"].items()) == 48511
    assert sum(batch["shape_counts"].values()) == batch["forwards"]
    assert batch["largest_multi_positions"] <= 2748
    guard = lines(evidence / "full-c.forward-guard.jsonl")
    assert len(guard) > 100 and max(g["consecutive_slow"] for g in guard) < 2
    rates = [g["forward_positions_s"] for g in guard]
    assert all(i["quick_check"] == "ok" for i in previous["indexes"])
    return {
        "full_batch": batch,
        "historical_singleton": previous,
        "forward_positions_s": 1000
        * batch["positions"]
        / batch["spans"]["forward"]["ms"],
        "relative_wall_decrease_pct": 100 * (1 - batch["wall_s"] / previous["wall_s"]),
        "relative_throughput_increase_pct": 100
        * (batch["passages_s"] / previous["passages_s"] - 1),
        "peak_footprint_delta_bytes": batch["peak_footprint_bytes"]
        - previous["peak_footprint_bytes"],
        "guard_samples": len(rates),
        "guard_median_positions_s": statistics.median(rates),
    }


if __name__ == "__main__":
    print(json.dumps(summarize(Path(sys.argv[1]).resolve()), indent=2, sort_keys=True))

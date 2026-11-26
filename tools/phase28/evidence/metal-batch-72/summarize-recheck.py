"""Summarize and validate the alternate interleaving, including host-state samples."""

import importlib.util
import json
import statistics
import sys
from pathlib import Path

reader = importlib.util.spec_from_file_location(
    "m4_verify", Path(__file__).with_name("verify-m4.py")
)
verify = importlib.util.module_from_spec(reader)
reader.loader.exec_module(verify)


def lines(path):
    with path.open() as capture:
        return list(map(json.loads, capture))


def summarize(root):
    ev = root / "evidence"
    rows = lines(ev / "recheck-headline.jsonl")
    matrix = {r["label"]: r for r in lines(ev / "recheck-matrix.jsonl")}
    states = {(r["label"], r["point"]): r for r in lines(ev / "host-state.jsonl")}
    order = [
        "re-b1",
        "re-c1",
        "re-a1",
        "re-a2",
        "re-c2",
        "re-b2",
        "re-b3",
        "re-c3",
        "re-a3",
    ]
    assert [r["label"] for r in rows] == order
    control = verify.read_index(root, "b1")
    comparisons = {}
    for row in rows:
        label = row["label"]
        source = matrix[label]
        assert len(row["before_idle_pct"]) == 2 and min(row["before_idle_pct"]) >= 95
        assert len(row["after_idle_pct"]) == 1 and (label, "after") in states
        assert row["positions"] == 110157 and row["forward_calls"] == (
            55 if row["route"] == "c" else 219
        )
        assert source["spans"]["forward"]["calls"] == row["forward_calls"]
        assert (
            abs(
                row["forward_positions_s"]
                - row["positions"] * 1000 / source["spans"]["forward"]["ms"]
            )
            < 1e-8
        )
        data = verify.read_index(root, label)
        cmp = verify.compare(control, data)
        assert cmp["equal_512"] == cmp["equal_768"] == 219, (label, cmp)
        comparisons[label] = cmp
    measures = (
        "wall_s",
        "passages_s",
        "forward_positions_s",
        "peak_rss_bytes",
        "peak_footprint_bytes",
    )
    medians = {
        a: {
            k: statistics.median(r[k] for r in rows if r["route"] == a)
            for k in measures
        }
        for a in "abc"
    }
    medians["batch_wall_gain_pct"] = 100 * (
        1 - medians["c"]["wall_s"] / medians["b"]["wall_s"]
    )
    medians["batch_rate_gain_pct"] = 100 * (
        medians["c"]["passages_s"] / medians["b"]["passages_s"] - 1
    )
    return {"medians": medians, "trials": rows, "comparisons": comparisons}


if __name__ == "__main__":
    print(json.dumps(summarize(Path(sys.argv[1]).resolve()), indent=2, sort_keys=True))

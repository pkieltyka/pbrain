"""Verify #77 receipts and print a Markdown table."""

import argparse
import json
import statistics
from pathlib import Path


def load(path):
    return json.loads(path.read_text())


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("capture", type=Path)
    root = parser.parse_args().capture
    summary = load(root / "summary.json")
    assert summary["corpus"] == [
        "2cbd38cde77fdc6f1d67949e50efd40d261d9f0a79d6bda5a759a854218db490",
        [4500, 500],
        50_000_000,
    ]
    fingerprint = summary["index_fingerprint"]
    for section, n in (("shared", 43642), ("personal", 4869)):
        before = summary["index"][section]
        assert before["passages"] == n and before["quick_check"] == "ok"
        assert before["manifest"]["fingerprint"] == fingerprint
        assert before["manifest"]["cuda_precision_request"] == "fp16"
    assert load(root / "indexes-after.json") == summary["index"]
    for arm in ("cpu", "gpu"):
        status = load(root / f"{arm}-ready-status" / "stdout")
        assert status["model"] == "embeddinggemma-2-q8-0" and status["model_present"]
        assert {s["section"]: s["items"] for s in status["sections"]} == {
            "shared": 4500,
            "personal": 500,
        }
        for row in status["sections"]:
            assert (
                row["indexed"] == row["items"]
                and not row.get("stale")
                and not row.get("unreadable")
            )
    assert len(list(root.glob("search-*"))) == 48
    assert len(summary["samples"]) == 8
    print(
        "| Binary | View | Query | Five samples (ms) | Median (ms) | Range (ms) | <1 s | Peak RSS (MiB) |"
    )
    print("| --- | --- | --- | --- | ---: | ---: | ---: | ---: |")
    rankings = {}
    expected_keys = {
        (a, v, q)
        for a in ("cpu", "gpu")
        for v in ("shared", "personal")
        for q in ("exact", "paraphrase")
    }
    for row in summary["samples"]:
        key = row["arm"], row["view"], row["case"]
        expected_keys.remove(key)
        samples, rss = [], []
        for i in range(6):
            stage = root / (
                "search-{}-{}-{}-{}".format(row["view"], row["case"], row["arm"], i)
            )
            receipt = load(stage / "receipt.json")
            assert receipt["exit"] == 0 and receipt["stop_reason"] is None
            assert not (stage / "stderr").read_bytes()
            results = [
                [r["id"], r.get("semantic", False)]
                for r in load(stage / "stdout")["results"]
            ]
            assert results == row["results"]
            match_key = row["view"], row["case"]
            assert results == rankings.setdefault(match_key, results)
            if i:
                samples.append(receipt["wall_s"] * 1000)
                rss.append(receipt["peak_rss_kib"] / 1024)
        median = statistics.median(samples)
        low, high = min(samples), max(samples)
        under = sum(t < 1000 for t in samples)
        peak = max(rss)
        assert (samples, median, [low, high], under, peak) == (
            row["samples_ms"],
            row["median_ms"],
            row["range_ms"],
            row["under_1000_ms"],
            row["peak_rss_mib"],
        )
        measurements = ", ".join(f"{t:.1f}" for t in samples)
        print(
            f"| {row['arm'].upper()} | {row['view']} | {row['case']} | {measurements} "
            f"| {median:.1f} | {low:.1f}–{high:.1f} | {under}/5 | {peak:,.1f} |"
        )
    assert not expected_keys


if __name__ == "__main__":
    main()

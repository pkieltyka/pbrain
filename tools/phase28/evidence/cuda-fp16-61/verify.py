"""Recompute the archived #61 rates, vector comparisons and retrieval gates."""
import csv
import json
import math
import statistics
import struct
import sys
from pathlib import Path


def read(path):
    return json.loads(path.read_text())


def lines(path):
    return [json.loads(line) for line in path.read_text().splitlines()]


def forwards(path):
    calls = [event for event in lines(path) if event["span"] == "forward"]
    assert calls and all(1 <= c["inputs"] <= 256 and 1 <= c["total_positions"] <= 8192 for c in calls), path
    return calls


def cosine(left, right):
    a, b = [struct.unpack("<768f", bytes.fromhex(blob)) for blob in (left, right)]
    assert all(math.isfinite(x) for x in a + b)
    an, bn = [math.sqrt(math.fsum(x*x for x in v)) for v in (a, b)]
    assert abs(an - 1) < 1e-5 and abs(bn - 1) < 1e-5
    return math.fsum(x*y for x, y in zip(a, b)) / (an * bn)


def speed(root, row):
    rates = {}
    for precision in ("fp32", "fp16"):
        trials = []
        for repeat in (1, 2):
            path = root / f"{row}-{precision}-{repeat}"
            report = read(path / "summary.json")
            assert len(report["fixture_sha256"]) == 64
            assert report["fixture_sha256"] == read(root/"fixture.json")["digested_path_sha256"]
            quiet = lines(path / "quiet.jsonl")
            assert len(quiet) >= 2 and all(s["host_busy_pct"] <= 4 and not s["nvidia_compute"] for s in quiet[-2:])
            forward = forwards(path / "trace.jsonl")
            passed = sum(f["inputs"] for f in forward)
            positions = sum(f["total_positions"] for f in forward)
            assert (passed, positions, len(forward)) == (219, 110157, 14)
            assert all(f["device"] == "cuda:0" and not f.get("failed") for f in forward)
            assert lines(path / "trace.jsonl")[-1]["accelerator_attaches"] == 1
            with (path / "gpu.csv").open() as log:
                device_peak = max(int(sample[1]) for sample in csv.reader(log))
            assert device_peak == report["device_peak_mib"]
            rate = positions / report["wall_s"]
            assert math.isclose(rate, report["positions_per_s"])
            trials.append(rate)
        rates[precision] = statistics.median(trials)
        adapter_events = root / "adapter" / f"{row}-{precision}" / "trace.jsonl"
        calls = forwards(adapter_events)
        assert (sum(c["inputs"] for c in calls), sum(c["total_positions"] for c in calls), len(calls)) == (200, 100066, 13)
        elapsed = lines(adapter_events)[-1]["spans"]["slice_embed"]["ms"]
        print(row, precision, f"{rates[precision]:.1f}/s sync, {1e3*100066/elapsed:.1f}/s replay")
    print(row, f"{rates['fp16']/rates['fp32']:.3f}x sync")


def parity(root, row):
    source = root / "quality" / row
    report = read(source/"parity-report.json")
    modes = ("cpu", "fp32", "fp16", "fp16-split")
    vectors = {mode: lines(source/"parity"/mode/"qualified-vectors.jsonl") for mode in modes}
    assert all(len(v) == 59 for v in vectors.values())
    for pair in report["comparisons"]:
        a, b = (vectors[pair[mode]] for mode in ("left", "right"))
        cosines = []
        identical = 0
        for left, right in zip(a, b):
            assert {k: v for k, v in left.items() if k != "vector"} == {k: v for k, v in right.items() if k != "vector"}
            ident = left["vector"] == right["vector"]
            identical += int(ident)
            cosines.append(cosine(left["vector"], right["vector"]))
        assert identical == pair["identical"] == (59 if pair["right"] == "fp16-split" else 0)
        assert min(cosines) >= 0.999 and math.isclose(min(cosines), pair["minimum"], rel_tol=0, abs_tol=5e-15)
        assert math.isclose(statistics.median(cosines), pair["median"], rel_tol=0, abs_tol=5e-15)
        print(row, pair["left"], pair["right"], f"minimum {min(cosines):.12f}, identical {identical}/59")
    assert len(report["identical_vector_cache_files"]) == 59
    split = forwards(source/"parity/fp16-split/sync-trace.jsonl")
    assert [c["total_positions"] for c in split] == [7712, 3669, 3711, 332, 1559]
    assert [c.get("failed", False) for c in split] == [True, False, False, False, False]


def retrieval(root, row, questions):
    source = root/"quality"/row
    ranks = read(source/"ranks.json")
    assert len(ranks) == 77
    qualified = [q for q in questions if q["kind"] == "answerable" or q["half"] == "held-out"]
    assert set(ranks) == {q["id"] for q in qualified}
    for q in qualified:
        found = ranks[q["id"]]
        for result in found.values():
            expected_rank = next((i for i, hit in enumerate(result["ids"], 1) if hit in q.get("expect", [])), None)
            assert expected_rank == result["expected_rank"]
        for variant in ("hybrid", "combined"):
            assert found[f"fp32-{variant}"] == found[f"fp16-{variant}"]
    aggregate = read(source/"rank-summary.json")
    assert not aggregate["changes"] and not aggregate["failures"]
    for variant in ("hybrid", "combined"):
        key = f"fp16-{variant}"
        answer_ranks = [ranks[q["id"]][key]["expected_rank"] for q in qualified if q["kind"] == "answerable"]
        empty = sum(not ranks[q["id"]][key]["ids"] for q in qualified if q["kind"] != "answerable")
        assert (sum(r == 1 for r in answer_ranks), sum(r is not None and r <= 3 for r in answer_ranks),
                sum(r is not None for r in answer_ranks), empty) == (39, 42, 42, 30)
    print(row, "all ranked IDs unchanged, 39 first / 42 top-three / 42 on page, 30 abstentions")


def main():
    root = Path(sys.argv[1])
    fixture = Path(__file__).resolve().parents[4] / "internal/semantic/calibrate/testdata/retrieval/questions.json"
    for row in ("google", "q8"):
        speed(root, row)
        parity(root, row)
        retrieval(root, row, read(fixture))
        a, b, c = [record for record in read(root/"rollout.json") if record["row"] == row]
        assert (a["selector"], b["selector"], c["selector"]) == ("cuda-eg2-fp16", "cuda", None)
        assert a["manifest"]["fingerprint"] == c["manifest"]["fingerprint"] != b["manifest"]["fingerprint"]
        assert c["result"]["passages_reused"] == 1 and c["result"]["passages_embedded"] == 0
    print("Qualification capture verified")


if __name__ == "__main__":
    main()

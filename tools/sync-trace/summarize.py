"""Summarize one trace and UTC nvidia-smi capture in a run directory."""
import argparse
import csv
from datetime import datetime, timezone
import json
from pathlib import Path
import statistics


def gpu_samples(path, forwards):
    intervals = sorted(
        (f["run"] / 1e9 + f["start_ms"] / 1e3,
         f["run"] / 1e9 + (f["start_ms"] + f["ms"]) / 1e3)
        for f in forwards
    )
    samples = []
    interval = 0
    with path.open() as fp:
        for timestamp, memory, utilization in csv.reader(fp):
            instant = datetime.strptime(timestamp.strip(), "%Y/%m/%d %H:%M:%S.%f")
            seconds = instant.replace(tzinfo=timezone.utc).timestamp()
            while interval < len(intervals) and intervals[interval][1] <= seconds:
                interval += 1
            in_forward = interval < len(intervals) and intervals[interval][0] <= seconds
            samples.append((int(memory), int(utilization), in_forward))
    factors = [util for _, util, inside in samples if inside]
    return {
        "sample_count": len(samples),
        "forward_samples": len(factors),
        "forward_gpu_mean_pct": statistics.mean(factors) if factors else None,
        "forward_gpu_median_pct": statistics.median(factors) if factors else None,
        "peak_gpu_mib": max(memory for memory, _, _ in samples),
    }


def summarize(directory):
    trace = [json.loads(line) for line in (directory / "trace.jsonl").read_text().splitlines()]
    summary = trace[-1]
    if summary["span"] != "summary" or any(event["run"] != summary["run"] for event in trace):
        raise ValueError(f"{directory}: expected one complete capture")
    forwards = [event for event in trace if event["span"] == "forward" and event["device"].startswith("cuda:")]
    if not forwards:
        raise ValueError(f"{directory}: no CUDA forwards")
    finished = [event for event in forwards if not event.get("failed")]
    seconds = sum(event["ms"] for event in forwards) / 1e3
    inputs = sum(event["inputs"] for event in finished)
    positions = sum(event["total_positions"] for event in finished)
    rates = {
        "successful_inputs": inputs,
        "successful_positions": positions,
        "failed_forwards": len(forwards) - len(finished),
        "forward_seconds": seconds,
        "forward_positions_per_second": positions / seconds,
        "forward_passages_per_second": inputs / seconds,
        "wall_positions_per_second": positions / (summary["ms"] / 1e3),
    }
    if embed := summary["spans"].get("slice_embed"):
        rates["slice_embed_seconds"] = embed["ms"] / 1e3
        rates["slice_embed_positions_per_second"] = positions / rates["slice_embed_seconds"]
    return {**summary, **rates, **gpu_samples(directory / "gpu.csv", forwards)}


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("capture", type=Path)
    args = parser.parse_args()
    print(json.dumps(summarize(args.capture), indent=2))

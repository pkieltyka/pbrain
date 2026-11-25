"""Recompute #69 device metrics and same-file CPU parity from the raw capture."""
import hashlib
import json
import math
from pathlib import Path
import sqlite3
import statistics
import struct
import sys

ROOT = Path(sys.argv[1]).resolve()
SQLITE = ("index_read", "sqlite_lookup", "sqlite_write", "sqlite_commit", "sqlite_checkpoint")
SPANS = ("model_verify", "model_decode", "device_attach", "forward", "sqlite")


def index(stage, dim, n):
    directory = stage / "docs/brain/.pbrain/index"
    manifest = json.loads((directory / "manifest").read_text())
    db = directory / manifest["db"]
    with sqlite3.connect(db.as_uri() + "?mode=ro&immutable=1", uri=True) as con:
        assert con.execute("PRAGMA quick_check").fetchall() == [("ok",)], db
        items = con.execute("SELECT * FROM item ORDER BY path").fetchall()
        rows = con.execute(
            "SELECT item.path, passage.ord, passage.start, passage.stop, passage.digest, passage.vector "
            "FROM passage JOIN item ON passage.item_id = item.id ORDER BY item.path, passage.ord"
        ).fetchall()
    assert len(items) == (22 if n == 219 else 32), stage
    assert len(rows) == n and all(len(row[-1]) == 4 * dim for row in rows), stage
    return manifest, items, rows


def cosine(a, b, dim):
    x, y = struct.unpack(f"<{dim}f", a), struct.unpack(f"<{dim}f", b)
    assert all(math.isfinite(v) for v in (*x, *y))
    return math.fsum(p*q for p, q in zip(x, y, strict=True)) / math.sqrt(
        math.fsum(p*p for p in x) * math.fsum(q*q for q in y)
    )


def metrics(stage, n, label, wall, device_mib=None, rss=None, footprint=None, dim=768):
    trace = [json.loads(line) for line in (stage / "trace.jsonl").read_text().splitlines()]
    return record(stage / "project", stage.name, n, label, wall, trace, device_mib, rss, footprint, dim)


def record(project, label, n, device, wall, trace, device_mib, rss, footprint, dim):
    model, items, rows = index(project, dim, n)
    assert trace[-1]["span"] == "summary"
    forwards = [r for r in trace if r.get("span") == "forward"]
    assert sum(f["inputs"] for f in forwards) == n and all(not f.get("failed") for f in forwards), project
    assert {f["device"] for f in forwards} == {device}, project
    spans = trace[-1]["spans"]
    span_ms = {k: spans.get(k, {}).get("ms", 0.0) for k in SPANS if k != "sqlite"}
    span_ms["sqlite"] = sum(spans.get(k, {}).get("ms", 0.0) for k in SQLITE)
    metrics = {
        "label": label,
        "wall_seconds": wall,
        "passages_per_second": n / wall,
        "peak_device_MiB": device_mib,
        "peak_process_rss_MiB": None if rss is None else rss / (1 << 20),
        "peak_process_footprint_MiB": None if footprint is None else footprint / (1 << 20),
        "span_ms": span_ms,
        "positions": sum(f["total_positions"] for f in forwards),
        "forward_calls": len(forwards),
        "attached": trace[-1]["accelerator_attaches"],
        "index_fingerprint": model["fingerprint"],
        "vectors_sha256": hashlib.sha256(b"".join(row[-1] for row in rows)).hexdigest(),
    }
    return metrics, (items, rows)


def compare(x, y, dim):
    assert x[0] == y[0], "different item metadata"
    mins, equal = 1.0, 0
    for a, b in zip(x[1], y[1], strict=True):
        assert a[:-1] == b[:-1], "different passage metadata"
        equal += a[-1] == b[-1]
        mins = min(mins, cosine(a[-1], b[-1], dim))
    return mins, equal


def jsonl(path):
    return [json.loads(line) for line in path.read_text().splitlines()]


def linux(reading):
    name, n = reading["model"], reading["count"]
    peak = reading["device_peak_bytes"]
    return metrics(ROOT / "linux-comparison" / reading["path"], n,
                   reading["device"].split(":")[0] + ":0" if peak is not None else "cpu",
                   reading["wall_s"], None if peak is None else peak / (1 << 20),
                   dim=384 if name == "bge" else 768)


def mac(reading, directory):
    label, n = reading["label"], reading["passages_embedded"]
    trace = jsonl(ROOT / directory / "evidence" / f"{label}.trace.jsonl")
    project = ROOT / directory / "runs" / label
    return record(project, label, n, reading["device"], reading["wall_s"], trace, None,
                  reading["peak_rss_bytes"], reading["peak_footprint_bytes"], 384 if reading.get("model") == "bge" else 768)


def cuda(reading):
    m, n = reading["model"], reading["passages"]
    arm = ROOT / f"{n}-{m}-{reading['precision']}-{reading['trial']}-068"
    return metrics(arm, n, "cuda:0", reading["wall_s"], reading["device_peak_mib"])


def cell(host, profile, model, n, runs, reference):
    assert len(runs) == 2
    dim = 384 if model == "bge" else 768
    minimums = [compare(r[1], reference, dim)[0] for r in runs]
    repeat_cosine, repeat_equal = compare(runs[0][1], runs[1][1], dim)
    assert min(minimums) >= 0.999 and repeat_equal == n, (host, profile, model, n, minimums, repeat_equal)
    values = [r[0] for r in runs]
    peaks = [r["peak_device_MiB"] for r in values if r["peak_device_MiB"] is not None]
    return {
        "host": host, "profile": profile, "model": model, "passages": n, "samples": values,
        "wall_s_median": statistics.median(r["wall_seconds"] for r in values),
        "passages_s_median": statistics.median(r["passages_per_second"] for r in values),
        "device_MiB_median": statistics.median(peaks) if peaks else None,
        "device_MiB_max": max(peaks) if peaks else None,
        "span_ms_median": {k: statistics.median(r["span_ms"][k] for r in values) for k in SPANS},
        "same_file_cpu_cosine_min": min(minimums),
        "repeat_cosine_min": repeat_cosine,
        "repeat_equal": repeat_equal,
    }


cpu_samples = jsonl(ROOT / "cpu-matrix.jsonl")
vulkan_samples = [r for r in jsonl(ROOT / "vulkan-matrix.jsonl") if "model" in r]
cuda_samples = [r for r in jsonl(ROOT / "cuda-matrix.jsonl") if r.get("version") == "068"]
metal_samples = jsonl(ROOT / "mac-run.jsonl")
m4_samples = jsonl(ROOT / "mac-matrix.jsonl")
result = []
for n in (219, 59):
    linux_cpu = {model: [linux(r) for r in cpu_samples if r["model"] == model and r["count"] == n]
                 for model in ("bge", "q8", "q6", "google")}
    for model, precision in (("google", "fp16"), ("q8", "fp16"), ("q8", "fp32"), ("q6", "fp32")):
        samples = [cuda(r) for r in cuda_samples if r["model"] == model and
                   r["precision"] == precision and r["passages"] == n]
        result.append(cell("pax", "cuda-" + precision, model, n, samples, linux_cpu[model][0][1]))
    for model in ("q8", "q6"):
        samples = [linux(r) for r in vulkan_samples if r["model"] == model and r["count"] == n]
        result.append(cell("pax", "vulkan", model, n, samples, linux_cpu[model][0][1]))
    for model in ("bge", "q8"):
        result.append(cell("pax", "cpu", model, n, linux_cpu[model], linux_cpu[model][0][1]))

    m4_cpu = {model: [mac(r, "mac-expanded") for r in m4_samples if r["model"] == model and r["count"] == n and not r["profile"]]
              for model in ("bge", "q8")}
    for profile in ("metal-eg2-combined-v1", "metal-eg2-stock-v1"):
        if n == 219:
            samples = [mac(r, "mac-base") for r in metal_samples if r["profile"] == profile and
                       r["label"] in ("stock-1", "stock-2", "default-1", "default-2")]
        else:
            samples = [mac(r, "mac-expanded") for r in m4_samples if r["profile"] == profile and r["count"] == n]
        result.append(cell("m4", profile, "q8", n, samples, m4_cpu["q8"][0][1]))
    for model in ("bge", "q8"):
        result.append(cell("m4", "cpu", model, n, m4_cpu[model], m4_cpu[model][0][1]))
print(json.dumps(result, indent=2, sort_keys=True))

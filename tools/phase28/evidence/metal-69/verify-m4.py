"""Verify native Q8 comparisons using the captured immutable SQLite generations."""
import collections
import hashlib
import json
import math
from pathlib import Path
import sqlite3
import statistics
import struct
import sys

ROOT = Path(sys.argv[1]).resolve()
EVIDENCE = ROOT / "evidence"
CELLS = ("stock-1", "default-1", "default-2", "stock-2", "cpu", "migration")
EXPECTED = {"stock-1": "metal-eg2-stock-v1", "stock-2": "metal-eg2-stock-v1",
            "default-1": "metal-eg2-combined-v1", "default-2": "metal-eg2-combined-v1", "cpu": ""}


def load_json(directory, name):
    return json.loads((directory / name).read_text())


def read_index(arm):
    directory = ROOT / "runs" / arm / "docs/brain/.pbrain/index"
    manifest = load_json(directory, "manifest")
    db = directory / manifest["db"]
    with sqlite3.connect(db.as_uri() + "?mode=ro&immutable=1", uri=True) as con:
        assert con.execute("PRAGMA quick_check").fetchall() == [("ok",)], arm
        items = con.execute("SELECT * FROM item ORDER BY path").fetchall()
        passages = con.execute("SELECT * FROM passage ORDER BY item_id, ord").fetchall()
    assert len(items) == 22 and len(passages) == 219, arm
    vectors = [p[-1] for p in passages]
    assert all(isinstance(v, bytes) and len(v) == 768 * 4 for v in vectors)
    return items, [p[:-1] for p in passages], vectors


def cosine(x, y):
    aa, bb = math.fsum(v*v for v in x), math.fsum(v*v for v in y)
    return math.fsum(a*b for a, b in zip(x, y, strict=True)) / math.sqrt(aa*bb)


def compare(left, right):
    assert left[0:2] == right[0:2], "different item/passage metadata"
    values = [tuple(struct.unpack("<768f", v) for v in pair) for pair in zip(left[2], right[2], strict=True)]
    assert all(math.isfinite(x) for pair in values for v in pair for x in v)
    return {"bit_equal": sum(a == b for a, b in zip(left[2], right[2], strict=True)),
            "min_cosine_768": min(cosine(a, b) for a, b in values),
            "min_cosine_512": min(cosine(a[:512], b[:512]) for a, b in values)}


data = {arm: read_index(arm) for arm in CELLS}
pairs = [
    ("stock-1", "stock-2"), ("default-1", "default-2"), ("stock-1", "default-1"),
    ("stock-1", "cpu"), ("default-1", "cpu"), ("stock-1", "migration"),
]
comparisons = {f"{a}/{b}": compare(data[a], data[b]) for a, b in pairs}
for pair in ("stock-1/stock-2", "default-1/default-2", "stock-1/migration"):
    assert comparisons[pair]["bit_equal"] == 219, pair
for pair in ("stock-1/cpu", "default-1/cpu"):
    assert min(comparisons[pair]["min_cosine_768"], comparisons[pair]["min_cosine_512"]) >= 0.999, pair

queries = load_json(ROOT, "queries.json")
assert len(queries) == 8
ranks = {arm: load_json(EVIDENCE, f"{arm}.ranks.json") for arm in ("stock-1", "default-1", "cpu")}
rank_differences = [i + 1 for i, values in enumerate(zip(*ranks.values(), strict=True)) if any(v != values[0] for v in values)]
assert any(ranks["stock-1"]), "all eight queries were empty"
assert not rank_differences, (rank_differences, ranks)

timings = collections.defaultdict(list)
for arm, profile in EXPECTED.items():
    r = load_json(EVIDENCE, f"{arm}.summary.json")
    assert r["profile"] == profile and r["passages_embedded"] == r["forward_calls"] == 219
    assert r["positions"] == 110157 and min(r["idle_pct"]) >= 95
    timings[profile or "cpu"].append(r)
for label, embed_count, reuse_count in (("migration-combined", 219, 0), ("noop-combined", 0, 0), ("migration-stock", 0, 219)):
    r = load_json(EVIDENCE, f"{label}.summary.json")
    assert r["passages_embedded"] == embed_count and r["passages_reused"] == reuse_count
    assert r["profile"] == (EXPECTED["stock-1"] if label == "migration-stock" else EXPECTED["default-1"])

summary = {
    "fixture": load_json(ROOT, "fixture-219.json")["digested_path_sha256"],
    "queries_sha256": hashlib.sha256((ROOT / "queries.json").read_bytes()).hexdigest(),
    "queries": len(queries), "rank_changes": rank_differences, "ranks": ranks,
    "medians": {arm: {
        "wall_s": statistics.median(r["wall_s"] for r in runs),
        "passages_per_s": statistics.median(r["passages_per_s"] for r in runs),
        "peak_footprint_bytes": max(r["peak_footprint_bytes"] for r in runs if r["peak_footprint_bytes"] is not None),
        "peak_rss_bytes": max(r["peak_rss_bytes"] for r in runs),
    } for arm, runs in timings.items()},
    "comparisons": comparisons,
}
print(json.dumps(summary, indent=2, sort_keys=True))

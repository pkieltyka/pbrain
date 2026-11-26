"""Verify and summarize the #72 M4 comparison from captured indexes and receipts."""

import hashlib
import json
import math
import sqlite3
import statistics
import struct
import sys
from pathlib import Path

ARMS = ("a1", "b1", "c1", "c2", "b2", "a2", "cpu")
COUNT = 219


def read_index(root, arm):
    directory = root / "runs" / arm / "docs/brain/.pbrain/index"
    manifest = json.loads((directory / "manifest").read_text())
    db = directory / manifest["db"]
    with sqlite3.connect(db.as_uri() + "?mode=ro&immutable=1", uri=True) as con:
        assert con.execute("PRAGMA quick_check").fetchall() == [("ok",)], arm
        items = con.execute("SELECT * FROM item ORDER BY path").fetchall()
        rows = con.execute("SELECT * FROM passage ORDER BY item_id, ord").fetchall()
    assert len(items) == 22 and len(rows) == COUNT, arm
    assert all(isinstance(r[-1], bytes) and len(r[-1]) == 768 * 4 for r in rows), arm
    return (items, [r[:-1] for r in rows]), [r[-1] for r in rows], manifest


def prefix(values):
    p = values[:512]
    norm = math.sqrt(math.fsum(v * v for v in p))
    return struct.pack("<512f", *(v / norm for v in p))


def cosine(a, b):
    cross = math.fsum(x * y for x, y in zip(a, b, strict=True))
    return cross / math.sqrt(math.fsum(x * x for x in a) * math.fsum(y * y for y in b))


def compare(left, right):
    assert left[0] == right[0], "different item/passage metadata"
    vectors_l, vectors_r = left[1], right[1]
    pairs = [
        (struct.unpack("<768f", a), struct.unpack("<768f", b))
        for a, b in zip(vectors_l, vectors_r, strict=True)
    ]
    assert all(math.isfinite(x) for p in pairs for vs in p for x in vs)
    return {
        "equal_768": sum(a == b for a, b in zip(vectors_l, vectors_r, strict=True)),
        "equal_512": sum(prefix(a) == prefix(b) for a, b in pairs),
        "min_cosine_768": min(cosine(a, b) for a, b in pairs),
        "min_cosine_512": min(cosine(a[:512], b[:512]) for a, b in pairs),
    }


def summary(root):
    ev = root / "evidence"
    data = {arm: read_index(root, arm) for arm in ARMS}
    pairs = {}
    for a in ARMS[:-1]:
        v = compare(data[a], data["cpu"])
        assert min(v["min_cosine_768"], v["min_cosine_512"]) >= 0.999, (a, v)
        pairs[a + "/cpu"] = v
    for a, b in [
        ("b1", "c1"),
        ("b2", "c2"),
        ("b1", "b2"),
        ("c1", "c2"),
        ("a1", "a2"),
        ("a1", "b1"),
        ("a2", "b2"),
    ]:
        v = compare(data[a], data[b])
        pairs[a + "/" + b] = v
        if a[0] != "a" or b[0] == "a":
            assert v["equal_768"] == v["equal_512"] == COUNT, (a, b, v)
        if a[0] != "a":
            assert data[a][2]["fingerprint"] == data[b][2]["fingerprint"]
    assert data["a1"][2]["fingerprint"] != data["b1"][2]["fingerprint"]
    ranks = {a: json.loads((ev / f"{a}.ranks.json").read_text()) for a in ARMS}
    assert (
        any(ranks["cpu"])
        and len(ranks["cpu"]) == 8
        and all(v == ranks["cpu"] for v in ranks.values())
    ), ranks
    with (ev / "matrix.jsonl").open() as capture:
        rows = [json.loads(l) for l in capture]
    assert [r["label"] for r in rows] == [*ARMS, "noop-off", "noop-on"]
    for r in rows:
        assert min(r["idle_pct"]) >= 95, r["label"]
        if r["label"].startswith("noop-"):
            assert (
                not r["forward_calls"]
                and not r["passages_embedded"]
                and not r["passages_reused"]
            )
            assert r["spans"]["device_attach"]["calls"] == 1
            continue
        assert r["passages_embedded"] == COUNT and r["positions"] == 110157
        forwards = r["forward_calls"]
        assert forwards < COUNT if r["batch"] else forwards == COUNT
    spans = (
        "model_verify",
        "model_decode",
        "device_attach",
        "forward",
        "pack",
        "sqlite_lookup",
        "sqlite_write",
        "sqlite_commit",
        "sqlite_checkpoint",
    )
    medians = {}
    for name in ("a", "b", "c", "cpu"):
        subset = [
            r
            for r in rows
            if r["label"].startswith(name) and (name == "cpu" or r["label"] != "cpu")
        ]
        medians[name] = {
            "wall_s": statistics.median(r["wall_s"] for r in subset),
            "passages_s": statistics.median(r["passages_s"] for r in subset),
            "forward_calls": [r["forward_calls"] for r in subset],
            "peak_rss_bytes": max(r["peak_rss_bytes"] for r in subset),
            "peak_footprint_bytes": max(r["peak_footprint_bytes"] for r in subset),
            "spans_ms": {
                s: statistics.median(r["spans"].get(s, {}).get("ms", 0) for r in subset)
                for s in spans
            },
        }
    return {
        "fixture": json.loads((root / "fixture-219.json").read_text())[
            "digested_path_sha256"
        ],
        "queries_sha256": hashlib.sha256(
            (root / "queries.json").read_bytes()
        ).hexdigest(),
        "medians": medians,
        "comparisons": pairs,
        "ranks": ranks,
    }


if __name__ == "__main__":
    print(json.dumps(summary(Path(sys.argv[1]).resolve()), indent=2, sort_keys=True))

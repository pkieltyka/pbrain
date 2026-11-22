#!/usr/bin/env python3
import filecmp
import hashlib
import json
import sqlite3
import sys
from contextlib import closing
from itertools import zip_longest
from pathlib import Path


def open_manifest(brain, section):
    if section == "personal":
        brain /= "personal"
    root = brain / ".pbrain" / "index"
    manifest = json.loads((root / "manifest").read_text())
    db = root / manifest.pop("db")
    return manifest, db


def digest_of(path):
    with path.open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def connect(path):
    return sqlite3.connect(path.as_uri() + "?mode=ro&immutable=1", uri=True)


def compare(section, base_brain, candidate_brain):
    left_manifest, left_db = open_manifest(base_brain, section)
    right_manifest, right_db = open_manifest(candidate_brain, section)
    left_sha, right_sha = digest_of(left_db), digest_of(right_db)
    for m, db in zip((left_manifest, right_manifest), (left_db, right_db)):
        assert m["size"] == db.stat().st_size, db
    for key in ("format", "fingerprint", "model", "dim"):
        assert left_manifest[key] == right_manifest[key], (section, key)
    meta_equal = left_manifest == right_manifest
    print(json.dumps({
        "section": section, "baseline_db": str(left_db), "candidate_db": str(right_db),
        "baseline_sha256": left_sha, "candidate_sha256": right_sha,
        "database_bytes_equal": filecmp.cmp(left_db, right_db, shallow=False),
        "manifest_fields_except_generated_name_equal": meta_equal,
        "baseline_manifest": left_manifest, "candidate_manifest": right_manifest,
    }))
    with closing(connect(left_db)) as left, closing(connect(right_db)) as right:
        assert list(left.execute("PRAGMA quick_check")) == [("ok",)], left_db
        assert list(right.execute("PRAGMA quick_check")) == [("ok",)], right_db
        expected = (4500, 43642) if section == "shared" else (500, 4869)
        rows = (
            ("sqlite_schema", "SELECT type, name, tbl_name, sql FROM sqlite_schema ORDER BY type, name", None),
            ("meta", "SELECT * FROM meta ORDER BY key", 4),
            ("item", "SELECT * FROM item ORDER BY path", expected[0]),
            ("passage", "SELECT * FROM passage ORDER BY item_id, ord", expected[1]),
        )
        for name, statement, count in rows:
            digest = hashlib.sha256()
            seen = 0
            for i, pair in enumerate(zip_longest(left.execute(statement), right.execute(statement))):
                a, b = pair
                if a != b:
                    raise ValueError(f"{section} {name} row {i + 1} differs")
                digest.update(repr(a).encode("utf-8") + b"\n")
                seen += 1
            if count is not None and seen != count:
                raise ValueError(f"{section} {name} count {seen}, expected {count}")
            print(json.dumps({"section": section, "table": name, "equal_rows": seen,
                              "row_sha256": digest.hexdigest()}))
    print(json.dumps({"section": section, "baseline_quick_check": "ok",
                      "candidate_quick_check": "ok"}))


if __name__ == "__main__":
    for section in ("shared", "personal"):
        compare(section, *(Path(p).resolve() / "docs" / "brain" for p in sys.argv[1:]))

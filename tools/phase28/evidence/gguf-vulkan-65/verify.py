#!/usr/bin/env python3
"""Independent read-only verifier for the pbrain #65 device evidence.

It opens the published SQLite indexes read-only, re-derives the pinned model
identities, and checks counts, metadata, unit vectors, CPU/Vulkan cosine,
same-device layout receipts and the fit guards. It never runs a model or opens
a device.

  python3 verify.py --out <evidence directory>
"""

import argparse
import datetime as dt
import hashlib
import json
import math
import re
import sqlite3
import struct
import sys
from pathlib import Path

GIB = 1 << 30
PIN_UUID = "vulkan:000000000c0000000000000000000000"
ALIAS = "vulkan:0"
VENDOR_DEVICE = "1002:7590"
ITEMS, PASSAGES, DIM = 32, 59, 768
COSINE_FLOOR = 0.999
LOAD_CAP = 4.0  # owner-approved 2026-11-23 for this descriptive sync
HOST_NEED_MB = 8 * 1024
ATTRIBUTED_CAP = 4 * GIB
CHILD_SWAP_CAP = 512 * (1 << 20)
FIXTURE_LIST_SHA256 = "77bab592f89a8975f17ed31aebb10414c1c940d7f0592395dd693a0a83b81dd1"
FIXTURE_PATHS_SHA256 = "3726e16e116d7db52fd2da7ce9145a940a3f7ef6ec797e83b0732084b947b892"
MODELS = {
    "q8": {
        "name": "embeddinggemma-2-q8-0",
        "file": "embeddinggemma-2-Q8_0.gguf",
        "size": 309855520,
        "sha": "6f1bd4ac6c5df7444f9cca7ca36cafe6cfa34cd6f49fefb1e0b4be8143aed8bc",
    },
    "q6": {
        "name": "embeddinggemma-2-ud-q6-k-xl",
        "file": "embeddinggemma-2-UD-Q6_K_XL.gguf",
        "size": 248812032,
        "sha": "dc84f042bc3ffbe6122b3b38b2fb299d66f5f37b276c9d458db85d4a4f1e6ba3",
    },
}
DB_NAME = re.compile(r"^[0-9a-f]{16}\.db$")
HEX64 = re.compile(r"^[0-9a-f]{64}$")


class Checker:
    def __init__(self, out):
        self.out = Path(out).resolve()
        self.failures = []
        self.metrics = {}

    def fail(self, message):
        self.failures.append(message)

    def require(self, condition, message):
        if not condition:
            self.fail(message)
        return condition

    def read(self, path):
        try:
            return json.loads(Path(path).read_text())
        except Exception as e:  # noqa: BLE001 - evidence absence is a failure, not a crash
            self.fail(f"{path}: {e}")
            return {}

    def sha(self, path):
        h = hashlib.sha256()
        with open(path, "rb") as f:
            for block in iter(lambda: f.read(1 << 20), b""):
                h.update(block)
        return h.hexdigest()

    def read_pins(self, path):
        pins = {}
        for line in Path(path).read_text().splitlines():
            if not line:
                continue
            digest, rel = line.split("  ", 1)
            pins[rel] = digest
        return pins

    def identity(self, name, filename, digest):
        fp = hashlib.sha256(f"{name}\n{filename}\n{digest}\n{DIM}\ngemma2-1".encode()).hexdigest()
        index = hashlib.sha256(f"pbrain-index\n1\n{fp}".encode()).hexdigest()
        h = hashlib.sha256(struct.pack("<Q", len(filename)) + filename.encode() + bytes.fromhex(digest))
        return fp, index, f"{name}-{h.hexdigest()[:16]}"

    def read_index(self, db):
        uri = f"file:{db}?mode=ro"
        with sqlite3.connect(uri, uri=True) as conn:
            quick = conn.execute("PRAGMA quick_check").fetchone()
            if quick != ("ok",):
                self.fail(f"{db}: quick_check {quick}")
                return [], []
            items = conn.execute("SELECT path, content FROM item ORDER BY path").fetchall()
            rows = conn.execute(
                "SELECT item.path, passage.ord, passage.start, passage.stop, passage.digest, passage.vector "
                "FROM passage JOIN item ON passage.item_id = item.id ORDER BY item.path, passage.ord").fetchall()
        return items, rows

    def check_cell(self, key, device):
        cell = self.out / "cells" / f"{key}-{device}"
        if not self.require(cell.is_dir(), f"missing cell {cell.name}"):
            return None
        result = self.read(cell / "result.json")
        cellinfo = self.read(cell / "cell.json")
        sync = self.read(cell / "sync.json")
        self.require(result.get("returncode") == 0, f"{cell.name}: returncode {result.get('returncode')}")
        self.require(result.get("stop") is None, f"{cell.name}: stop {result.get('stop')}")
        wall = result.get("process_wall_ns", 0)
        self.require(isinstance(wall, int) and wall > 0, f"{cell.name}: no process wall")
        self.require(result.get("pageout_delta") == 0, f"{cell.name}: pageout {result.get('pageout_delta')}")
        self.require(not result.get("swap_void"), f"{cell.name}: swap void {result.get('swap_void')}")
        self.require(result.get("launch_load1", 99) <= LOAD_CAP, f"{cell.name}: launch load {result.get('launch_load1')}")
        self.require(result.get("launch_available_mb", 0) >= HOST_NEED_MB,
                     f"{cell.name}: available {result.get('launch_available_mb')} MiB")
        self.require(result.get("peak_attributed_bytes", 0) <= ATTRIBUTED_CAP,
                     f"{cell.name}: attributed peak over cap")
        self.require(result.get("max_device_vram_delta", 0) <= ATTRIBUTED_CAP,
                     f"{cell.name}: device delta over cap")
        self.require(result.get("peak_swap_bytes", 0) <= CHILD_SWAP_CAP,
                     f"{cell.name}: child swap {result.get('peak_swap_bytes')}")
        expected_device = "cpu" if device == "cpu" else "vulkan"
        self.require(result.get("device") == expected_device, f"{cell.name}: device {result.get('device')}")
        if device == "vulkan":
            self.require(result.get("attributed_samples", 0) > 0, f"{cell.name}: no attribution")
            self.require(result.get("peak_vram_bytes", 0) > 0, f"{cell.name}: no device peak")
            self.require(result.get("selector") == PIN_UUID, f"{cell.name}: selector {result.get('selector')}")

        self.require(cellinfo.get("model") == MODELS[key]["name"], f"{cell.name}: model {cellinfo.get('model')}")
        self.require(cellinfo.get("model_sha256") == MODELS[key]["sha"], f"{cell.name}: model pin")
        self.require(sync.get("model") == MODELS[key]["name"], f"{cell.name}: sync model {sync.get('model')}")
        sections = sync.get("sections") or []
        if not self.require(len(sections) == 1, f"{cell.name}: {len(sections)} sections"):
            return None
        section = sections[0]
        self.require(section.get("section") == "shared", f"{cell.name}: section {section.get('section')}")
        self.require(section.get("items") == ITEMS, f"{cell.name}: items {section.get('items')}")
        self.require(section.get("passages_embedded") == PASSAGES,
                     f"{cell.name}: embedded {section.get('passages_embedded')}")
        self.require(section.get("passages_reused") == 0, f"{cell.name}: reused {section.get('passages_reused')}")
        self.require(section.get("device") == ("cpu" if device == "cpu" else ALIAS),
                     f"{cell.name}: reported device {section.get('device')}")

        index = cell / "brain/.pbrain/index"
        manifest = self.read(index / "manifest")
        fp, index_fp, _ = self.identity(MODELS[key]["name"], MODELS[key]["file"], MODELS[key]["sha"])
        self.require(manifest.get("format") == 3, f"{cell.name}: manifest format {manifest.get('format')}")
        self.require(manifest.get("model") == MODELS[key]["name"], f"{cell.name}: manifest model")
        self.require(manifest.get("dim") == DIM, f"{cell.name}: manifest dim")
        self.require(manifest.get("fingerprint") == index_fp, f"{cell.name}: manifest fingerprint")
        name = manifest.get("db", "")
        self.require(bool(DB_NAME.match(str(name))), f"{cell.name}: db name {name!r}")
        db = index / name
        if not self.require(db.is_file(), f"{cell.name}: missing {name}"):
            return None
        self.require(db.stat().st_size == manifest.get("size"), f"{cell.name}: db size")
        items, rows = self.read_index(db)
        self.require(len(items) == ITEMS, f"{cell.name}: {len(items)} items")
        self.require(all(len(content) == 32 for _, content in items), f"{cell.name}: item digest size")
        self.require(len(rows) == PASSAGES, f"{cell.name}: {len(rows)} passages")
        vector_sha = hashlib.sha256()
        for path, ord_, start, stop, digest, blob in rows:
            self.require(len(digest) == 32, f"{cell.name}: passage digest size at {path}/{ord_}")
            if len(blob) != 4 * DIM:
                self.fail(f"{cell.name}: vector width {len(blob)} at {path}/{ord_}")
                continue
            vector = struct.unpack(f"<{DIM}f", blob)
            if not all(map(math.isfinite, vector)):
                self.fail(f"{cell.name}: nonfinite vector at {path}/{ord_}")
            norm = math.sqrt(math.fsum(v * v for v in vector))
            if abs(norm - 1) > 1e-5:
                self.fail(f"{cell.name}: unit length {norm} at {path}/{ord_}")
            vector_sha.update(blob)
        cell_metrics = self.metrics.setdefault(cell.name, {})
        cell_metrics.update({
            "wall_s": wall / 1e9, "passages_per_s": PASSAGES * 1e9 / wall,
            "peak_host_rss_bytes": result.get("peak_host_rss_bytes"),
            "peak_attributed_bytes": result.get("peak_attributed_bytes"),
            "max_device_vram_delta": result.get("max_device_vram_delta"),
            "peak_vram_bytes": result.get("peak_vram_bytes"),
            "vector_sha256": vector_sha.hexdigest(), "items": len(items), "passages": len(rows),
            "quick_check": "ok",
        })
        return {"items": items, "rows": rows, "manifest": manifest, "vector_sha": vector_sha.hexdigest(),
                "fp": fp, "index_fp": index_fp}

    def verify(self):
        manifest = self.read(self.out / "manifest.json")
        build = self.read(self.out / "build.json")
        preflight = self.read(self.out / "preflight.json")
        device = self.read(self.out / "device.json")
        if not manifest:
            self.fail("missing manifest.json")
            return self.finish()

        self.require(manifest.get("issue") == 65, "manifest issue")
        self.require(manifest.get("pinned_uuid") == PIN_UUID, "manifest UUID")
        self.require(manifest.get("alias") == ALIAS, "manifest alias")
        self.require(manifest.get("vendor_device") == VENDOR_DEVICE, "manifest vendor/device")
        self.require(manifest.get("thresholds", {}).get("load_cap") == LOAD_CAP, "manifest load cap")
        self.require(manifest.get("thresholds", {}).get("cosine_floor") == COSINE_FLOOR, "manifest cosine floor")
        self.require(manifest.get("thresholds", {}).get("items") == ITEMS, "manifest items")
        self.require(manifest.get("thresholds", {}).get("passages") == PASSAGES, "manifest passages")
        self.require(manifest.get("models") == MODELS, "manifest models")
        self.require(device.get("persistent") == PIN_UUID and device.get("id") == ALIAS
                     and device.get("backend") == "vulkan"
                     and "Radeon RX 9060 XT" in device.get("name", ""), "device inventory row")
        self.require(preflight.get("pci") == "0000:0c:00.0", "preflight PCI")
        self.require(preflight.get("vendor") == 0x1002 and preflight.get("device") == 0x7590, "preflight identity")

        commit = manifest.get("commit")
        self.require(bool(commit), "manifest commit")
        self.require(build.get("commit") == commit, "build commit")
        self.require(build.get("clean") is True, "build dirty")
        for name in ("pbrain", "gemma2-vulkan.test"):
            binary = build.get("binaries", {}).get(name, {})
            path = self.out / "bin" / name
            if self.require(path.is_file(), f"missing binary {name}"):
                self.require(binary.get("sha256") == self.sha(path), f"{name}: binary hash")
            info = binary.get("build_info", "")
            self.require("vulkan" in info, f"{name}: build info lacks vulkan")
            if commit:
                self.require(commit in info or "vcs.revision" not in info, f"{name}: build revision")

        list_path = self.out / "fixture.SHA256"
        paths_path = self.out / "fixture-paths.txt"
        expected_fixture = None
        if list_path.is_file() and paths_path.is_file():
            self.require(self.sha(list_path) == FIXTURE_LIST_SHA256, "fixture list hash")
            self.require(self.sha(paths_path) == FIXTURE_PATHS_SHA256, "fixture paths hash")
            expected_fixture = self.read_pins(list_path)
            paths = [line for line in paths_path.read_text().splitlines() if line]
            self.require(len(expected_fixture) == ITEMS, f"fixture pins {len(expected_fixture)}")
            self.require(len(paths) == ITEMS and set(paths) == set(expected_fixture),
                         "fixture path list and pins disagree")
            fixture = manifest.get("fixture") or {}
            self.require(fixture.get("sha256") == FIXTURE_LIST_SHA256, "manifest fixture list")
            self.require(fixture.get("paths_sha256") == FIXTURE_PATHS_SHA256, "manifest fixture paths")
        else:
            self.fail("missing fixture pins in evidence")

        data = {}
        for key in MODELS:
            for device_kind in ("cpu", "vulkan"):
                data[(key, device_kind)] = self.check_cell(key, device_kind)

        for key in MODELS:
            name = MODELS[key]["name"]
            _, index_fp, folder = self.identity(name, MODELS[key]["file"], MODELS[key]["sha"])
            cpu = data.get((key, "cpu"))
            gpu = data.get((key, "vulkan"))
            if not cpu or not gpu:
                continue
            self.require(cpu["manifest"].get("fingerprint") == index_fp, f"{key}: CPU index fingerprint")
            self.require(gpu["manifest"].get("fingerprint") == index_fp, f"{key}: Vulkan index fingerprint")
            if [(p, c) for p, c in cpu["items"]] != [(p, c) for p, c in gpu["items"]]:
                self.fail(f"{key}: CPU and Vulkan item paths/content differ")
            if [row[:5] for row in cpu["rows"]] != [row[:5] for row in gpu["rows"]]:
                self.fail(f"{key}: CPU and Vulkan passage identity differs")
            minimum = 1.0
            worst = None
            for left, right in zip(cpu["rows"], gpu["rows"], strict=True):
                a = struct.unpack(f"<{DIM}f", left[5])
                b = struct.unpack(f"<{DIM}f", right[5])
                cosine = math.fsum(x * y for x, y in zip(a, b, strict=True)) / (
                    math.sqrt(math.fsum(x * x for x in a)) * math.sqrt(math.fsum(y * y for y in b)))
                if cosine < minimum:
                    minimum, worst = cosine, (left[0], left[1])
            self.require(minimum >= COSINE_FLOOR, f"{key}: CPU/Vulkan cosine {minimum}")
            self.metrics.setdefault(f"{key}", {}).update({
                "min_cpu_cosine": minimum, "worst": worst, "index_fingerprint": index_fp, "cache_folder": folder,
                "vector_sha_cpu": cpu["vector_sha"], "vector_sha_vulkan": gpu["vector_sha"],
            })

        q8 = data.get(("q8", "cpu"))
        q6 = data.get(("q6", "cpu"))
        if q8 and q6:
            self.require(q8["index_fp"] != q6["index_fp"], "Q8 and Q6 share an index fingerprint")
            self.require(q8["vector_sha"] != q6["vector_sha"], "Q8 and Q6 share a vector digest")

        layout = self.read(self.out / "layout/result.json")
        self.require(layout.get("returncode") == 0, f"layout: returncode {layout.get('returncode')}")
        self.require(layout.get("stop") is None, f"layout: stop {layout.get('stop')}")
        self.require(layout.get("pageout_delta") == 0, "layout: pageout")
        self.require(not layout.get("swap_void"), "layout: swap void")
        self.require(layout.get("peak_attributed_bytes", 0) <= ATTRIBUTED_CAP, "layout: attributed cap")
        self.require(layout.get("attributed_samples", 0) > 0, "layout: no attribution")
        self.require(layout.get("peak_vram_bytes", 0) > 0, "layout: no device peak")
        self.require(layout.get("peak_attributed_bytes", 0) > 0, "layout: no attributed bytes")
        for key in MODELS:
            receipt = self.read(self.out / "layout" / f"{MODELS[key]['name']}.json")
            self.require(receipt.get("model") == MODELS[key]["name"], f"layout {key}: model")
            self.require(receipt.get("persistent") == PIN_UUID, f"layout {key}: persistent")
            self.require(receipt.get("alias") == ALIAS, f"layout {key}: alias")
            self.require(receipt.get("passages") == PASSAGES, f"layout {key}: passages")
            differing = receipt.get("differing") or {}
            for label in ("packed", "split17", "reversed", "tiny-budget"):
                self.require(differing.get(label) == 0, f"layout {key}: {label} differs {differing.get(label)}")
            self.require(receipt.get("min_cpu_cosine_768", 0) >= COSINE_FLOOR, f"layout {key}: cosine 768")
            self.require(receipt.get("min_cpu_cosine_512", 0) >= COSINE_FLOOR, f"layout {key}: cosine 512")
            self.metrics.setdefault(key, {}).update({
                "layout_differing": differing,
                "layout_min_cosine_768": receipt.get("min_cpu_cosine_768"),
                "layout_min_cosine_512": receipt.get("min_cpu_cosine_512"),
            })

        if expected_fixture is not None:
            for key in MODELS:
                for kind in ("cpu", "vulkan"):
                    cell = self.out / "cells" / f"{key}-{kind}"
                    if not cell.is_dir():
                        continue
                    for rel, digest in expected_fixture.items():
                        path = cell / "brain" / rel
                        if not self.require(path.is_file(), f"{cell.name}: missing fixture {rel}"):
                            continue
                        self.require(self.sha(path) == digest, f"{cell.name}: fixture {rel} digest")

        return self.finish()

    def finish(self):
        ok = not self.failures
        verification = {"ok": ok, "generated": dt.datetime.now(dt.timezone.utc).isoformat(),
                        "failures": self.failures, "metrics": self.metrics}
        (self.out / "verification.json").write_text(json.dumps(verification, indent=2) + "\n")
        (self.out / "verification.log").write_text(
            ("PASS" if ok else "FAIL") + "\n" + "\n".join(self.failures) + ("\n" if self.failures else ""))
        if ok:
            for cell, m in sorted(self.metrics.items()):
                if "wall_s" in m:
                    print(f"PASS {cell}: {m['items']} items / {m['passages']} passages, "
                          f"{m['wall_s']:.3f} s, {m['passages_per_s']:.3f} passages/s, "
                          f"RSS {m['peak_host_rss_bytes']} B, attributed {m['peak_attributed_bytes']} B")
                else:
                    print(f"PASS {cell}: min cosine {m.get('min_cpu_cosine')} layout {m.get('layout_differing')}")
        else:
            print("FAIL")
            for failure in self.failures:
                print("  " + failure)
        return ok


def verify(out):
    return Checker(out).verify()


def main(argv=None):
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--out", required=True)
    a = p.parse_args(argv)
    return 0 if verify(a.out) else 1


if __name__ == "__main__":
    raise SystemExit(main())

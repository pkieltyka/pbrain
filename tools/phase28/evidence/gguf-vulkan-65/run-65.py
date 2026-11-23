#!/usr/bin/env python3
"""pbrain #65 device acceptance runner.

Builds the reviewed pbrain candidate with the Vulkan tag, runs fresh CPU and
Vulkan SQLite syncs for Q8_0 and UD-Q6_K_XL on the pinned Radeon, runs the
same-device layout test, and writes an evidence directory for verify.py.

Run it only under the repository's exclusive timing lock, after #76 releases the
device:

  python3 <goinfer>/scripts/timing_lock.py run --exclusive --label pbrain-65-device -- \
      python3 tools/phase28/evidence/gguf-vulkan-65/run-65.py --out <dir> ...

This file performs no work when merely reviewed; the device stages are explicit.
"""

import argparse
import datetime as dt
import hashlib
import json
import os
import re
import shutil
import subprocess
import sys
import time
from pathlib import Path

GIB = 1 << 30
PIN_UUID = "vulkan:000000000c0000000000000000000000"
ALIAS = "vulkan:0"
BDF = "0000:0c:00.0"
VENDOR, DEVICE = 0x1002, 0x7590
HOST_NEED_MB = 8 * 1024
RADEON_NEED = 2 * GIB
ATTRIBUTED_CAP = 4 * GIB
CHILD_SWAP_CAP = 512 * (1 << 20)
CELL_DEADLINE_S = 90
LAYOUT_DEADLINE_S = 600
QUIET_DEADLINE_S = 600
LOAD_CAP = 4.0  # owner-approved 2026-11-23 for this descriptive sync
COSINE_FLOOR = 0.999
ITEMS, PASSAGES = 32, 59

MODELS = {
    "q8": {
        "name": "embeddinggemma-2-q8-0",
        "file": "embeddinggemma-2-Q8_0.gguf",
        "size": 309855520,
        "sha": "6f1bd4ac6c5df7444f9cca7ca36cafe6cfa34cd6f49fefb1e0b4be8143aed8bc",
        "env": "PBRAIN_TEST_GEMMA2_Q8",
    },
    "q6": {
        "name": "embeddinggemma-2-ud-q6-k-xl",
        "file": "embeddinggemma-2-UD-Q6_K_XL.gguf",
        "size": 248812032,
        "sha": "dc84f042bc3ffbe6122b3b38b2fb299d66f5f37b276c9d458db85d4a4f1e6ba3",
        "env": "PBRAIN_TEST_GEMMA2_Q6",
    },
}

RESET = re.compile(r"amdgpu.*(?:ring .*timeout|ring reset|GPU reset|device wedged|coredump)", re.I)
VALIDATION = re.compile(r"Validation Error|VUID-|SYNC-HAZARD")
LAYER_LOADED = re.compile(r"Insert instance layer .*VK_LAYER_KHRONOS_validation")
CLIENT_KEYS = ("drm-memory-vram", "drm-memory-gtt", "drm-memory-cpu")


def drm_totals(memory):
    """Sum vram, gtt and cpu across the process's DRM clients, or None."""
    if not memory:
        return None
    return [sum(v[i] for v in memory.values()) for i in range(len(CLIENT_KEYS))]


def now():
    return dt.datetime.now(dt.timezone.utc).isoformat()


def sha256(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for block in iter(lambda: f.read(1 << 20), b""):
            h.update(block)
    return h.hexdigest()


def write_json(path, obj):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(obj, indent=2) + "\n")


def read_json(path):
    return json.loads(Path(path).read_text())


def prop(name):
    return int((Path("/sys/bus/pci/devices") / BDF / name).read_text(), 0)


def vram_used():
    return prop("mem_info_vram_used")


def vram_total():
    return prop("mem_info_vram_total")


def pageouts():
    return int(dict(line.split() for line in Path("/proc/vmstat").read_text().splitlines())["pswpout"])


def status(pid):
    return dict(line.split(":", 1) for line in (Path("/proc") / str(pid) / "status").read_text().splitlines() if ":" in line)


def fdinfo(pid):
    out = {}
    try:
        descriptors = list((Path("/proc") / str(pid) / "fdinfo").iterdir())
    except (FileNotFoundError, ProcessLookupError):
        return out
    for descriptor in descriptors:
        try:
            data = dict(line.split(":", 1) for line in descriptor.read_text().splitlines() if ":" in line)
        except (FileNotFoundError, ProcessLookupError):
            continue
        if data.get("drm-pdev", "").strip() != BDF:
            continue
        client = data.get("drm-client-id", "").strip()
        if not client or not all(re.fullmatch(r"[0-9]+ KiB", data.get(k, "").strip()) for k in CLIENT_KEYS):
            continue
        out[client] = [int(data[k].split()[0]) * 1024 for k in CLIENT_KEYS]
    return out


def pbrain_active(ours=None):
    result = subprocess.run(
        ["pgrep", "-af", r"^([^ ]*/)?(pbrain[^ /]*|calibrate-cpu|calibrate-cuda|gemma2-dual\.test)( |$)"],
        capture_output=True, text=True)
    if result.returncode not in (0, 1):
        raise RuntimeError(result.stderr)
    return [line for line in result.stdout.splitlines() if int(line.split()[0]) != ours]


def desktop_map(path):
    if path is None:
        return {}
    return {int(pid): name for pid, name in read_json(path).items()}


def foreign_clients(pids, comm_of, desktop, ours=None):
    """PIDs on the render node that are not the exact baseline pid+comm pairs.

    A new PID is foreign even when its comm matches an approved name; only the
    explicit baseline map is exempt.
    """
    return {pid: comm_of[pid] for pid in pids
            if pid != ours and pid in comm_of and desktop.get(pid) != comm_of[pid]}


def competing_clients(desktop, ours=None):
    files = ("/dev/dri/by-path/pci-0000:0c:00.0-render", "/dev/dri/by-path/pci-0000:0c:00.0-card")
    result = subprocess.run(["fuser", *files], capture_output=True, text=True)
    if result.returncode not in (0, 1):
        raise RuntimeError(result.stderr)
    pids = [int(pid) for pid in result.stdout.split()]
    comm_of = {}
    for pid in pids:
        try:
            comm_of[pid] = (Path("/proc") / str(pid) / "comm").read_text().strip()
        except FileNotFoundError:
            continue
    return foreign_clients(pids, comm_of, desktop, ours)


def journal(*args):
    return subprocess.run(["journalctl", "--quiet", "--no-pager", "-k", *args],
                          check=True, capture_output=True, text=True).stdout


def journal_cursor():
    return json.loads(journal("-n", "1", "-o", "json"))["__CURSOR"]


def load1():
    return os.getloadavg()[0]


def available_mb():
    for line in Path("/proc/meminfo").read_text().splitlines():
        if line.startswith("MemAvailable:"):
            return int(line.split()[1]) / 1024
    raise RuntimeError("MemAvailable missing")


class Runner:
    def __init__(self, args):
        self.args = args
        self.out = Path(args.out).resolve()
        self.repo = Path(__file__).resolve().parents[4]
        self.script = Path(__file__).resolve().parent
        if not (self.repo / "go.mod").is_file() or not (
                self.repo / "internal/semantic/calibrate/testdata/retrieval/brain").is_dir():
            raise SystemExit(f"cannot resolve the pbrain repository root from {Path(__file__).resolve()}")
        self.goinfer = Path(args.goinfer_repo).resolve()
        self.desktop = desktop_map(args.desktop_clients)
        self.manifest = None

    # ---- shared ---------------------------------------------------------

    def need_scripts(self):
        for name in ("timing_lock.py", "void_rule.py"):
            if not (self.goinfer / "scripts" / name).is_file():
                raise SystemExit(f"missing {name} under --goinfer-repo {self.goinfer}")
        sys.path.insert(0, str(self.goinfer / "scripts"))
        global timing_lock, void_rule
        import timing_lock as _t
        import void_rule as _v
        timing_lock, void_rule = _t, _v

    def require_lock(self):
        holder = timing_lock.read_holder()
        token = os.environ.get("GOINFER_TIMING_LOCK")
        if not holder or not token or holder.get("token") != token or holder.get("mode") != "exclusive":
            raise SystemExit("run this under: timing_lock.py run --exclusive --label pbrain-65-device -- ...")

    def base_env(self):
        env = {k: v for k, v in os.environ.items() if not k.startswith("PBRAIN_")}
        for key in ("VK_DRIVER_FILES", "VK_ICD_FILENAMES", "VK_ADD_DRIVER_FILES", "DRI_PRIME",
                    "MESA_VK_DEVICE_SELECT", "VK_LOADER_DRIVERS_SELECT", "VK_LOADER_DRIVERS_DISABLE"):
            env.pop(key, None)
        env["LD_LIBRARY_PATH"] = str(self.args.vulkan_loader_dir) + ":/run/opengl-driver/lib"
        env["VK_ADD_LAYER_PATH"] = str(self.args.vulkan_layer_dir)
        env["VK_INSTANCE_LAYERS"] = "VK_LAYER_KHRONOS_validation"
        env["VK_LAYER_VALIDATE_SYNC"] = "1"
        env["VK_LOADER_DEBUG"] = "layer"
        return env

    def check_vectors(self):
        if not (Path(self.args.vulkan_loader_dir) / "libvulkan.so.1").is_file():
            raise SystemExit("the Vulkan loader directory lacks libvulkan.so.1")
        if not (Path(self.args.vulkan_layer_dir) / "VkLayer_khronos_validation.json").is_file():
            raise SystemExit("the Vulkan layer directory lacks VkLayer_khronos_validation.json")
        if not os.access(Path(self.args.vulkan_loader_dir) / "libvulkan.so.1", os.R_OK):
            raise SystemExit("unreadable Vulkan loader")

    def fixture(self):
        root = self.repo / "internal/semantic/calibrate/testdata/retrieval/brain"
        paths = [line for line in (self.script / "fixture-paths.txt").read_text().splitlines() if line]
        pins = {}
        for line in (self.script / "fixture.SHA256").read_text().splitlines():
            digest, rel = line.split("  ", 1)
            pins[rel] = digest
        if len(paths) != ITEMS or set(paths) != set(pins):
            raise SystemExit("fixture list and pins disagree")
        files = {}
        for rel in paths:
            data = (root / rel).read_bytes()
            if hashlib.sha256(data).hexdigest() != pins[rel]:
                raise SystemExit(f"fixture digest mismatch: {rel}")
            files[rel] = data
        return files, pins

    def wait_quiet(self):
        begun = time.monotonic()
        while True:
            snap = void_rule.snapshot()
            why = void_rule.start_void(snap, HOST_NEED_MB, cap=LOAD_CAP)
            free = vram_total() - vram_used()
            foreign = pbrain_active() or competing_clients(self.desktop)
            if not why and free >= RADEON_NEED and not foreign:
                return snap, free
            if time.monotonic() - begun > QUIET_DEADLINE_S:
                raise SystemExit(f"no quiet window after {QUIET_DEADLINE_S}s: {why or 'foreign work or Radeon fit'}")
            time.sleep(5)

    def admission(self):
        """The readings a cell starts with, rechecked immediately before spawn."""
        snap, _ = self.wait_quiet()
        live = void_rule.snapshot()
        why = void_rule.start_void(live, HOST_NEED_MB, cap=LOAD_CAP)
        free = vram_total() - vram_used()
        if why or free < RADEON_NEED or pbrain_active() or competing_clients(self.desktop):
            raise SystemExit(f"launch refused: {why or 'foreign work or Radeon fit'}")
        return live, free

    def reset_guard(self, cursor):
        return bool(RESET.search(journal("--after-cursor", cursor, "-o", "short-iso")))

    def preflight(self):
        if prop("vendor") != VENDOR or prop("device") != DEVICE:
            raise SystemExit("the Radeon PCI identity is not 1002:7590")
        snap, free = self.wait_quiet()
        if pbrain_active() or competing_clients(self.desktop):
            raise SystemExit("preflight saw foreign pbrain or Radeon work")
        cursor = journal_cursor()
        if self.reset_guard(cursor):
            raise SystemExit("recent amdgpu reset")
        write_json(self.out / "preflight.json", {
            "time": now(), "snapshot": snap, "radeon_free_bytes": free,
            "load1": load1(), "available_mb": available_mb(),
            "vendor": prop("vendor"), "device": prop("device"),
            "pci": BDF, "kernel": os.uname().release,
            "desktop_clients": {str(k): v for k, v in self.desktop.items()},
        })

    def inventory(self):
        env = self.base_env()
        cmd = [str(self.out / "bin" / "pbrain"), "devices", "--json"]
        result = subprocess.run(cmd, env=env, capture_output=True, text=True, timeout=30)
        (self.out / "inventory.stdout").write_bytes(result.stdout.encode())
        (self.out / "inventory.stderr").write_bytes(result.stderr.encode())
        result.check_returncode()
        rows = json.loads(result.stdout)["devices"]
        matches = [r for r in rows if r.get("backend") == "vulkan" and r.get("persistent") == PIN_UUID]
        if len(matches) != 1 or matches[0].get("id") != ALIAS or "Radeon RX 9060 XT" not in matches[0].get("name", ""):
            raise SystemExit(f"pinned Radeon UUID/alias not found: {rows}")
        write_json(self.out / "device.json", matches[0])
        return matches[0]

    # ---- build ----------------------------------------------------------

    def build(self):
        self.out.mkdir(parents=True, exist_ok=True)
        if (self.out / "bin").exists():
            raise SystemExit("--out already holds a build")
        head = subprocess.run(["git", "-C", str(self.repo), "rev-parse", "HEAD"],
                              check=True, capture_output=True, text=True).stdout.strip()
        dirty = subprocess.run(["git", "-C", str(self.repo), "status", "--porcelain"],
                               check=True, capture_output=True, text=True).stdout.strip()
        if dirty:
            raise SystemExit("the pbrain worktree is not clean; build only a reviewed commit")
        if self.args.commit and head != self.args.commit:
            raise SystemExit(f"HEAD {head} does not match --commit {self.args.commit}")
        self.args.commit = head
        go = subprocess.run(["go", "version"], check=True, capture_output=True, text=True).stdout.strip()
        env = dict(os.environ, GOWORK="off", CGO_ENABLED="0", GOOS="linux", GOARCH="amd64")
        (self.out / "bin").mkdir()
        pbrain = self.out / "bin" / "pbrain"
        testbin = self.out / "bin" / "gemma2-vulkan.test"
        cmd = ["go", "build", "-tags", "vulkan", "-o", str(pbrain), "./cmd/pbrain"]
        subprocess.run(cmd, cwd=self.repo, env=env, check=True)
        cmd2 = ["go", "test", "-c", "-tags", "vulkan", "-o", str(testbin), "./internal/embed/gemma2"]
        subprocess.run(cmd2, cwd=self.repo, env=env, check=True)
        info = {
            "commit": head, "clean": True, "go_version": go,
            "flags": {"GOWORK": "off", "CGO_ENABLED": "0", "GOOS": "linux", "GOARCH": "amd64", "tags": "vulkan"},
            "commands": [" ".join(cmd), " ".join(cmd2)],
            "binaries": {
                "pbrain": {"sha256": sha256(pbrain), "bytes": pbrain.stat().st_size},
                "gemma2-vulkan.test": {"sha256": sha256(testbin), "bytes": testbin.stat().st_size},
            },
        }
        for name, path in (("pbrain", pbrain), ("gemma2-vulkan.test", testbin)):
            info["binaries"][name]["build_info"] = subprocess.run(
                ["go", "version", "-m", str(path)], check=True, capture_output=True, text=True).stdout
        shutil.copyfile(self.script / "fixture.SHA256", self.out / "fixture.SHA256")
        shutil.copyfile(self.script / "fixture-paths.txt", self.out / "fixture-paths.txt")
        write_json(self.out / "build.json", info)
        print(json.dumps({"built": head, **info["binaries"]}, indent=2), flush=True)

    # ---- cells ----------------------------------------------------------

    def cell_env(self, cell, device):
        env = self.base_env()
        env.update(
            HOME=str(cell / "home"), XDG_CACHE_HOME=str(cell / "cache"),
            XDG_CONFIG_HOME=str(cell / "home/.config"), PBRAIN_CACHE=str(cell / "cache"),
            PBRAIN_TRACE=str(cell / "trace.jsonl"), GOMAXPROCS="4",
        )
        if device != "cpu":
            env["PBRAIN_GPU_MEMORY"] = "1024"
        return env

    def make_cell(self, cell, key, device, files, model_file):
        brain = cell / "brain"
        brain.mkdir(parents=True)
        for rel, data in files.items():
            path = brain / rel
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(data)
        (brain / "pbrain.toml").write_text("[semantic]\nmodel = " + json.dumps(MODELS[key]["name"]) + "\n")
        model_dir = cell / "model"
        model_dir.mkdir()
        target = model_dir / MODELS[key]["file"]
        try:
            os.link(model_file, target)
        except OSError:
            shutil.copyfile(model_file, target)
        if target.stat().st_size != MODELS[key]["size"] or sha256(target) != MODELS[key]["sha"]:
            raise SystemExit(f"{key}: model copy failed its pin")
        (brain / "personal.toml").write_text("model_path = " + json.dumps(str(model_dir)) + "\n")
        return brain

    def sample_loop(self, child, cell, baseline, pswpout, cursor, deadline):
        began = time.monotonic_ns()
        next_check = began
        peak_vram = peak_total = peak_delta = peak_swap = peak_rss = 0
        samples = 0
        stop = None
        detail = None
        with (cell / "samples.jsonl").open("w") as out:
            while child.poll() is None:
                moment = time.monotonic_ns()
                try:
                    memory = fdinfo(child.pid)
                    st = status(child.pid)
                except (FileNotFoundError, ProcessLookupError):
                    continue
                delta = max(0, vram_used() - baseline)
                swap = int(st.get("VmSwap", "0 kB").split()[0]) * 1024
                rss = int(st.get("VmHWM", "0 kB").split()[0]) * 1024
                peak_delta = max(peak_delta, delta)
                peak_swap = max(peak_swap, swap)
                peak_rss = max(peak_rss, rss)
                if totals := drm_totals(memory):
                    samples += 1
                    peak_vram = max(peak_vram, totals[0])
                    peak_total = max(peak_total, sum(totals))
                out.write(json.dumps({"t_ns": moment - began, "bytes": memory, "system_vram_delta": delta,
                                      "swap": swap, "rss": rss}) + "\n")
                if peak_total >= ATTRIBUTED_CAP or peak_delta >= ATTRIBUTED_CAP or peak_swap > CHILD_SWAP_CAP:
                    stop = "memory cap or child swap"
                    detail = {"peak_total": peak_total, "peak_delta": peak_delta, "peak_swap": peak_swap}
                elif pageouts() != pswpout:
                    stop = "host pageout"
                    detail = {"pswpout": pswpout, "now": pageouts()}
                elif moment - began >= deadline * 10**9:
                    stop = f"{deadline}-second application deadline"
                elif moment >= next_check:
                    next_check = moment + 500_000_000
                    active = pbrain_active(child.pid)
                    clients = competing_clients(self.desktop, child.pid)
                    if active or clients:
                        stop = "foreign pbrain or Radeon work"
                        detail = {"pbrain": active, "clients": {str(k): v for k, v in clients.items()}}
                    elif self.reset_guard(cursor):
                        stop = "amdgpu timeout or reset"
                if stop:
                    child.kill()
                    break
                time.sleep(0.005)
        if child.poll() is None:
            child.kill()
        child.wait()
        return {
            "process_wall_ns": time.monotonic_ns() - began, "stop": stop, "stop_detail": detail,
            "attributed_samples": samples, "peak_vram_bytes": peak_vram,
            "peak_attributed_bytes": peak_total, "max_device_vram_delta": peak_delta,
            "peak_swap_bytes": peak_swap, "peak_host_rss_bytes": peak_rss,
        }

    def run_cell(self, key, device, files, model_file):
        cell = self.out / "cells" / f"{key}-{device if device != 'cpu' else 'cpu'}"
        if cell.exists():
            raise SystemExit(f"cell exists: {cell}")
        cell.mkdir(parents=True)
        brain = self.make_cell(cell, key, device, files, model_file)
        selector = "cpu" if device == "cpu" else PIN_UUID
        env = self.cell_env(cell, device)
        cmd = [str(self.out / "bin" / "pbrain"), "--brain", str(brain), "sync", "--device", selector, "--json"]
        (cell / "command.txt").write_text(" ".join(cmd) + "\n")
        snap, free = self.admission()
        pswpout = pageouts()
        baseline = vram_used()
        cursor = journal_cursor()
        with (cell / "sync.json").open("wb") as stdout, (cell / "sync.stderr").open("wb") as stderr:
            child = subprocess.Popen(cmd, env=env, stdout=stdout, stderr=stderr)
            result = self.sample_loop(child, cell, baseline, pswpout, cursor, CELL_DEADLINE_S)
        kernel = journal("--after-cursor", cursor, "-o", "short-iso")
        (cell / "kernel.log").write_text(kernel)
        stderr_text = (cell / "sync.stderr").read_text(errors="replace")
        swap_void = void_rule.end_void(snap, void_rule.snapshot(), env={})
        sampler_stop, postflight_stop = result.pop("stop"), None
        if self.reset_guard(cursor) or (device != "cpu" and "VK_ERROR_DEVICE_LOST" in stderr_text):
            postflight_stop = "amdgpu timeout/reset or device loss"
        elif device != "cpu" and VALIDATION.search(stderr_text):
            postflight_stop = "Vulkan validation failed"
        elif device != "cpu" and not LAYER_LOADED.search(stderr_text):
            postflight_stop = "Vulkan validation layer missing"
        elif pageouts() != pswpout or swap_void or pbrain_active() or competing_clients(self.desktop):
            postflight_stop = "postflight pageout, swap, or foreign work"
        elif device != "cpu" and (not result["attributed_samples"] or not result["peak_vram_bytes"]):
            postflight_stop = "missing Radeon attribution"
        elif child.returncode:
            postflight_stop = f"sync exited {child.returncode}"
        result["sampler_stop"], result["postflight_stop"] = sampler_stop, postflight_stop
        result["stop"] = sampler_stop or postflight_stop
        result.update({
            "returncode": child.returncode, "device": device, "selector": selector,
            "launch_load1": snap.get("load1"), "launch_available_mb": snap.get("available_mb"),
            "launch_radeon_free_bytes": free, "pageout_delta": pageouts() - pswpout,
            "swap_void": swap_void,
        })
        write_json(cell / "result.json", result)
        write_json(cell / "cell.json", {
            "model": MODELS[key]["name"], "key": key, "device": device, "selector": selector,
            "fixture_sha256": sha256(self.script / "fixture.SHA256"),
            "model_file": MODELS[key]["file"], "model_sha256": MODELS[key]["sha"],
        })
        print(json.dumps({"cell": cell.name, **result}), flush=True)
        return result

    # ---- layout ---------------------------------------------------------

    def layout(self, q8_dir, q6_dir):
        cell = self.out / "layout"
        cell.mkdir(parents=True)
        env = self.base_env()
        env.update(
            HOME=str(cell / "home"), XDG_CACHE_HOME=str(cell / "cache"),
            XDG_CONFIG_HOME=str(cell / "home/.config"), PBRAIN_CACHE=str(cell / "cache"),
            GOMAXPROCS="4", PBRAIN_ALLOW_GEMMA2="1",
            PBRAIN_TEST_VULKAN_DEVICE=PIN_UUID, PBRAIN_GPU_MEMORY="1024",
            PBRAIN_TEST_GEMMA2_Q8=str(q8_dir), PBRAIN_TEST_GEMMA2_Q6=str(q6_dir),
            PBRAIN_VULKAN_LAYOUT_DIR=str(cell),
        )
        package = self.repo / "internal/embed/gemma2"
        fixture = (package / "../../semantic/calibrate/testdata/retrieval/brain").resolve()
        if not fixture.is_dir():
            raise SystemExit(f"layout fixture is missing from the package working directory: {fixture}")
        cmd = [str(self.out / "bin" / "gemma2-vulkan.test"),
               "-test.run", "^TestGemmaVulkanLayouts$", "-test.v", "-test.timeout", "600s"]
        snap, free = self.admission()
        pswpout = pageouts()
        baseline = vram_used()
        cursor = journal_cursor()
        with (cell / "stdout.log").open("wb") as stdout, (cell / "stderr.log").open("wb") as stderr:
            child = subprocess.Popen(cmd, cwd=package, env=env, stdout=stdout, stderr=stderr)
            result = self.sample_loop(child, cell, baseline, pswpout, cursor, LAYOUT_DEADLINE_S)
        kernel = journal("--after-cursor", cursor, "-o", "short-iso")
        (cell / "kernel.log").write_text(kernel)
        text = (cell / "stdout.log").read_text(errors="replace") + (cell / "stderr.log").read_text(errors="replace")
        swap_void = void_rule.end_void(snap, void_rule.snapshot(), env={})
        sampler_stop, postflight_stop = result.pop("stop"), None
        if self.reset_guard(cursor) or "VK_ERROR_DEVICE_LOST" in text:
            postflight_stop = "amdgpu timeout/reset or device loss"
        elif VALIDATION.search(text):
            postflight_stop = "Vulkan validation failed"
        elif not LAYER_LOADED.search(text):
            postflight_stop = "Vulkan validation layer missing"
        elif pageouts() != pswpout or swap_void or pbrain_active() or competing_clients(self.desktop):
            postflight_stop = "postflight pageout, swap, or foreign work"
        elif not result["attributed_samples"] or not result["peak_vram_bytes"]:
            postflight_stop = "missing Radeon attribution"
        elif child.returncode:
            postflight_stop = f"layout test exited {child.returncode}"
        result["sampler_stop"], result["postflight_stop"] = sampler_stop, postflight_stop
        result["stop"] = sampler_stop or postflight_stop
        result["cwd"] = str(package)
        result.update({"returncode": child.returncode, "pageout_delta": pageouts() - pswpout,
                       "swap_void": swap_void, "launch_load1": snap.get("load1"),
                       "launch_available_mb": snap.get("available_mb"), "launch_radeon_free_bytes": free})
        write_json(cell / "result.json", result)
        print(json.dumps({"cell": "layout", **result}), flush=True)
        return result

    # ---- stages ---------------------------------------------------------

    def stage_build(self):
        self.need_scripts()
        self.require_lock()
        self.build()

    def stage_qualify(self):
        self.need_scripts()
        self.require_lock()
        build = read_json(self.out / "build.json")
        if not self.args.commit:
            self.args.commit = build["commit"]
        if build.get("commit") != self.args.commit:
            raise SystemExit("build.json does not match the expected commit")
        self.check_vectors()
        files, _ = self.fixture()
        self.preflight()
        self.inventory()
        q8_file = Path(self.args.q8_file).resolve()
        q6_file = Path(self.args.q6_file).resolve()
        for key, path in (("q8", q8_file), ("q6", q6_file)):
            if path.stat().st_size != MODELS[key]["size"] or sha256(path) != MODELS[key]["sha"]:
                raise SystemExit(f"{key}: reviewed file does not match its pin")
        results = {}
        for key, path in (("q8", q8_file), ("q6", q6_file)):
            for device in ("cpu", "vulkan"):
                results[f"{key}-{device}"] = self.run_cell(key, device, files, path)
        q8_dir = self.out / "cells" / "q8-vulkan" / "model"
        q6_dir = self.out / "cells" / "q6-vulkan" / "model"
        results["layout"] = self.layout(q8_dir, q6_dir)
        write_json(self.out / "results.json", results)
        bad = [name for name, r in results.items() if r.get("stop") or r.get("returncode")]
        if bad:
            raise SystemExit("device stage did not pass: " + ", ".join(bad))

    def stage_verify(self):
        from verify import verify  # same directory
        ok = verify(self.out)
        if ok:
            files = sorted(p for p in self.out.rglob("*") if p.is_file() and p.name != "evidence.SHA256")
            lines = [f"{sha256(p)}  {p.relative_to(self.out).as_posix()}\n" for p in files]
            (self.out / "evidence.SHA256").write_text("".join(lines))
        raise SystemExit(0 if ok else 1)

    def manifest_write(self):
        self.manifest = {
            "issue": 65, "generated": now(), "commit": self.args.commit,
            "pinned_uuid": PIN_UUID, "alias": ALIAS, "pci": BDF,
            "vendor_device": f"{VENDOR:04x}:{DEVICE:04x}",
            "models": MODELS,
            "thresholds": {
                "load_cap": LOAD_CAP, "host_need_mb": HOST_NEED_MB,
                "radeon_need_bytes": RADEON_NEED, "attributed_cap_bytes": ATTRIBUTED_CAP,
                "child_swap_cap_bytes": CHILD_SWAP_CAP, "cell_deadline_s": CELL_DEADLINE_S,
                "layout_deadline_s": LAYOUT_DEADLINE_S, "quiet_deadline_s": QUIET_DEADLINE_S,
                "cosine_floor": COSINE_FLOOR, "items": ITEMS, "passages": PASSAGES,
            },
            "fixture": {"sha256": sha256(self.script / "fixture.SHA256"),
                        "paths_sha256": sha256(self.script / "fixture-paths.txt"), "notes": ITEMS},
            "binary": read_json(self.out / "build.json") if (self.out / "build.json").is_file() else None,
            "cells": ["q8-cpu", "q8-vulkan", "q6-cpu", "q6-vulkan"],
            "layout_receipts": [f"{key}.json" for key in MODELS],
            "commands": {
                "qualify": "timing_lock.py run --exclusive --label pbrain-65-device -- python3 run-65.py --stage qualify --out <out> ...",
                "verify": "python3 run-65.py --stage verify --out <out>",
            },
        }
        write_json(self.out / "manifest.json", self.manifest)

    def run(self):
        if self.args.stage in ("build", "all"):
            self.stage_build()
            self.manifest_write()
        if self.args.stage in ("qualify", "all"):
            self.stage_qualify()
            self.manifest_write()
        if self.args.stage in ("verify", "all"):
            self.stage_verify()


def main(argv=None):
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--stage", choices=("build", "qualify", "verify", "all"), default="all")
    p.add_argument("--out", required=True)
    p.add_argument("--commit", default=None, help="expected pbrain commit")
    p.add_argument("--goinfer-repo", default=os.environ.get("GOINFER_REPO", ""))
    p.add_argument("--q8-file", default=None)
    p.add_argument("--q6-file", default=None)
    p.add_argument("--desktop-clients", default=None,
                   help="exact baseline pid->comm map; only those pairs are exempt")
    p.add_argument("--vulkan-loader-dir", default=os.environ.get("VULKAN_LOADER_DIR", ""))
    p.add_argument("--vulkan-layer-dir", default=os.environ.get("VULKAN_LAYER_DIR", ""))
    a = p.parse_args(argv)
    if not a.goinfer_repo:
        p.error("--goinfer-repo or GOINFER_REPO is required")
    if a.stage in ("qualify", "all") and (not a.q8_file or not a.q6_file):
        p.error("--q8-file and --q6-file are required for the device stage")
    if a.stage != "verify" and (not a.vulkan_loader_dir or not a.vulkan_layer_dir):
        p.error("--vulkan-loader-dir and --vulkan-layer-dir are required")
    return Runner(a).run()


if __name__ == "__main__":
    raise SystemExit(main())

"""Render the full measurement matrix or the compact release-notes matrix."""
import json
from pathlib import Path
import sys

DATA = json.loads(Path(sys.argv[1]).read_text())
MODELS = {"bge": "BGE small", "q8": "Q8_0", "q6": "UD-Q6_K_XL", "google": "Google pack"}
PROFILES = {"cpu": "CPU", "cuda-fp16": "RTX PRO 6000, CUDA fp16", "cuda-fp32": "RTX PRO 6000, CUDA ordinary",
            "vulkan": "RX 9060 XT, Vulkan", "metal-eg2-combined-v1": "Metal default (combined)",
            "metal-eg2-stock-v1": "Metal forced stock"}
SPANS = ("model_verify", "model_decode", "device_attach", "forward", "sqlite")
HOSTS = (("pax", "Ryzen 9 7950X (Linux)"), ("m4", "Apple M4 (macOS)"))


def format_mib(value):
    return f"{value:,.0f}" if value is not None else "—"


def span_tuple(row):
    def fmt(key):
        value = row["span_ms_median"][key]
        return f"{value:.0f}" if value >= 1 else f"{value:.1f}" if value > 0 else "—"
    return " / ".join(map(fmt, SPANS))


def brief(host):
    print("| Device and writer | Model | 219 wall (s) / p/s | 59 wall (s) / p/s | Peak device MiB, 219 / 59 |")
    print("| --- | --- | ---: | ---: | ---: |")
    for row in DATA:
        if row["host"] != host or row["passages"] != 219:
            continue
        short, = [r for r in DATA if (r["host"],r["profile"],r["model"],r["passages"]) ==
                  (host,row["profile"],row["model"],59)]
        cells = [PROFILES[row["profile"]], MODELS[row["model"]]]
        cells += [f'{r["wall_s_median"]:.2f} / {r["passages_s_median"]:.2f}' for r in (row, short)]
        cells += [" / ".join(format_mib(r["device_MiB_max"]) for r in (row, short))]
        print("| " + " | ".join(cells) + " |")


def detailed(host):
    print("| Passages | Device and writer | Model | Wall (s) | Passages/s | Peak device (MiB) | Verify / decode / attach / forward / SQLite (ms) | Min CPU cosine |")
    print("| ---: | --- | --- | ---: | ---: | ---: | ---: | ---: |")
    for row in DATA:
        if row["host"] != host:
            continue
        cells = [str(row["passages"]), PROFILES[row["profile"]], MODELS[row["model"]],
                 f'{row["wall_s_median"]:.2f}', f'{row["passages_s_median"]:.2f}', format_mib(row["device_MiB_max"]),
                 span_tuple(row), f'{row["same_file_cpu_cosine_min"]:.12f}']
        print("| " + " | ".join(cells) + " |")


for name, heading in HOSTS:
    print("### " + heading + "\n")
    (brief if "--brief" in sys.argv else detailed)(name)
    print()

#!/usr/bin/env python3
"""Host control for the #65 foreign-client exemption.

A second PID whose comm matches an approved desktop client must still be
reported as foreign. This runs no model, device or build.
"""

import importlib.util
from pathlib import Path

spec = importlib.util.spec_from_file_location("run65", Path(__file__).with_name("run-65.py"))
run65 = importlib.util.module_from_spec(spec)
spec.loader.exec_module(run65)


def main():
    desktop = {100: "chrome", 101: "Xwayland"}
    comm_of = {100: "chrome", 101: "Xwayland", 102: "chrome"}
    got = run65.foreign_clients([100, 101, 102], comm_of, desktop)
    assert got == {102: "chrome"}, f"new similar-name PID was exempt: {got}"

    assert run65.foreign_clients([100], {100: "chrome"}, {100: "chrome"}, ours=100) == {}
    assert run65.foreign_clients([100], {100: "chrome"}, {}) == {100: "chrome"}
    assert run65.foreign_clients([100, 999], {100: "chrome"}, {100: "chrome"}) == {}
    print("PASS host control: only exact baseline pid+comm pairs are exempt; a new PID with the same comm is foreign")


if __name__ == "__main__":
    main()

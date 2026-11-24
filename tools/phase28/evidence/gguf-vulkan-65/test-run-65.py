#!/usr/bin/env python3
"""Host control for the #65 foreign-client exemption.

A second PID whose comm matches an approved desktop client must still be
reported as foreign. This runs no model, device or build.
"""

import importlib.util
import json
from pathlib import Path

spec = importlib.util.spec_from_file_location("run65", Path(__file__).with_name("run-65.py"))
run65 = importlib.util.module_from_spec(spec)
spec.loader.exec_module(run65)
spec = importlib.util.spec_from_file_location("verify65", Path(__file__).with_name("verify.py"))
verify65 = importlib.util.module_from_spec(spec)
spec.loader.exec_module(verify65)


def main():
    desktop = {100: "chrome", 101: "Xwayland"}
    comm_of = {100: "chrome", 101: "Xwayland", 102: "chrome"}
    got = run65.foreign_clients([100, 101, 102], comm_of, desktop)
    assert got == {102: "chrome"}, f"new similar-name PID was exempt: {got}"

    assert run65.foreign_clients([100], {100: "chrome"}, {100: "chrome"}, ours=100) == {}
    assert run65.foreign_clients([100], {100: "chrome"}, {}) == {100: "chrome"}
    assert run65.foreign_clients([100, 999], {100: "chrome"}, {100: "chrome"}) == {}
    assert run65.drm_totals({"a": [1, 2, 3], "b": [10, 20, 30]}) == [11, 22, 33]
    assert run65.drm_totals({}) is None

    index = Path("/tmp/pbrain-index")
    assert verify65.safe_db_path(index, "0123456789abcdef.db") == index / "0123456789abcdef.db"
    for bad in ("../outside.db", "/tmp/outside.db", "0123456789abcdef.db/../x", "", "ABCDEF0123456789.db"):
        assert verify65.safe_db_path(index, bad) is None, bad

    complete = {"peak_attributed_bytes": 1, "max_device_vram_delta": 2, "peak_swap_bytes": 0}
    for field in complete:
        assert verify65.numeric(complete, field) == complete[field], field
        missing = dict(complete)
        del missing[field]
        assert verify65.numeric(missing, field) is None, field
        text = dict(complete, **{field: "0"})
        assert verify65.numeric(text, field) is None, field
        boolean = dict(complete, **{field: True})
        assert verify65.numeric(boolean, field) is None, field
        negative = dict(complete, **{field: -1})
        assert verify65.numeric(negative, field) is None, field
        for raw in ("NaN", "Infinity", "-Infinity"):
            nonfinite = json.loads('{"x": ' + raw + '}')
            assert verify65.numeric(nonfinite, "x") is None, (field, raw)
    assert all(verify65.numeric(complete, field) is not None for field in complete)
    print("PASS host control: exact pid+comm exemptions, DRM aggregation, DB basename confinement and finite nonnegative guard fields")


if __name__ == "__main__":
    main()

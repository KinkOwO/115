#!/usr/bin/env python3
"""Decompile a list of addresses with Hex-Rays and dump readable pseudocode.

Generic helper for the P0 evidence pass: point it at any VA discovered by the
survey scripts and get a .c file plus a one line summary (size, callees).

Run inside IDA:

    idat -A -L<log> -S"analysis/dumps/ida_decompile_va.py" <work>.i64

Edit TARGETS below before each run; the names are only labels.
"""

import json
import os
from datetime import datetime

import ida_funcs
import ida_hexrays
import idautils

# (label, address) pairs to decompile.
TARGETS = [
    ("bin_resolve_1474cdd90", 0x1474CDD90),
    ("bin_resolve2_1474cde40", 0x1474CDE40),
    ("bin_1474d1bb0", 0x1474D1BB0),
    ("bin_1474d1c30", 0x1474D1C30),
    ("bin_1474cb4b0", 0x1474CB4B0),
    ("bin_1474cb500", 0x1474CB500),
    ("txt_resolve_1474cdc20", 0x1474CDC20),
    ("txt_resolve2_1474cdcb0", 0x1474CDCB0),
]

MAX_LINES = 900

MAX_LINES = 2500

MAX_LINES = 2500

MAX_LINES = 1500

MAX_LINES = 2500

MAX_LINES = 300

MAX_LINES = 1200

MAX_LINES = 900

MAX_LINES = 700

MAX_LINES = 1000

MAX_LINES = 900

MAX_LINES = 2500

MAX_LINES = 1200

MAX_LINES = 2500

MAX_LINES = 2500

MAX_LINES = 2000

MAX_LINES = 1400

MAX_LINES = 700
OUT_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "va-decompile")


def callees(ea):
    func = ida_funcs.get_func(ea)
    if func is None:
        return []
    out = {}
    for item in idautils.FuncItems(func.start_ea):
        for target in idautils.CodeRefsFrom(item, 0):
            target_func = ida_funcs.get_func(target)
            if target_func is not None and target_func.start_ea != func.start_ea:
                out[target_func.start_ea] = ida_funcs.get_func_name(target_func.start_ea)
    return [{"ea": hex(k), "name": v} for k, v in sorted(out.items())]


def main():
    if not os.path.isdir(OUT_DIR):
        os.makedirs(OUT_DIR)
    summary = {"generated": datetime.now().isoformat(timespec="seconds"), "targets": []}
    for label, ea in TARGETS:
        record = {"label": label, "ea": hex(ea), "func_name": ida_funcs.get_func_name(ea)}
        try:
            cfunc = ida_hexrays.decompile(ea)
            text = str(cfunc) if cfunc else ""
        except Exception as exc:
            text = "hexrays failed: %s" % exc
        lines = text.splitlines()
        record["lines"] = len(lines)
        if len(lines) > MAX_LINES:
            lines = lines[:MAX_LINES] + ["... (%d more lines)" % (record["lines"] - MAX_LINES)]
        path = os.path.join(OUT_DIR, "%s_%x.c" % (label, ea))
        with open(path, "w", encoding="utf-8") as handle:
            handle.write("\n".join(lines) + "\n")
        record["file"] = path
        record["callees"] = callees(ea)
        summary["targets"].append(record)
        print("[%s] %#x %s lines=%d -> %s" % (label, ea, record["func_name"], record["lines"], path))
    with open(os.path.join(OUT_DIR, "summary.json"), "w", encoding="utf-8") as handle:
        json.dump(summary, handle, indent=2, ensure_ascii=False)
    print("wrote %s" % os.path.join(OUT_DIR, "summary.json"))


if __name__ == "__main__":
    main()

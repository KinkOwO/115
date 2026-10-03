#!/usr/bin/env python3
"""Decompile the weekly-open gate (⑦) of the legion entry chain.

next73 pinned the chain: sub_142510A50 / sub_142511D10 run gates 1-7 and gate 7
is `sub_145695000(...) == 0` -> dstr 100088632 "This dungeon isn't open today."
That is the live blocker, so this pass decompiles:

  * both entry chain functions,
  * sub_145695000 itself (the named table lookup),
  * every callee of sub_145695000 one level deep (the actual table read),
  * plus the virtual methods behind gates 5/6 for context.

Flushed to disk after every function so an interrupted run keeps partial output.
"""

import json
import os
from datetime import datetime

import ida_funcs
import ida_hexrays
import idautils

OUT_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "weekly-gate")

TARGETS = [
    ("chain_142510A50", 0x142510A50),
    ("chain_142511D10", 0x142511D10),
    ("lookup_145695000", 0x145695000),
    ("chain_1424FE550_send2043", 0x1424FE550),
]

MAX_LINES = 900


def main():
    os.makedirs(OUT_DIR, exist_ok=True)
    queue = list(TARGETS)
    seen = {ea for _, ea in queue}

    # one level of callees of the lookup function
    f = ida_funcs.get_func(0x145695000)
    if f is not None:
        for item in idautils.FuncItems(f.start_ea):
            for target in idautils.CodeRefsFrom(item, 0):
                tf = ida_funcs.get_func(target)
                if tf is None or tf.start_ea in seen:
                    continue
                seen.add(tf.start_ea)
                queue.append(("callee_%x" % tf.start_ea, tf.start_ea))

    summary = {"generated": datetime.now().isoformat(timespec="seconds"), "targets": []}
    for label, ea in queue:
        record = {"label": label, "ea": hex(ea)}
        try:
            cfunc = ida_hexrays.decompile(ea)
            text = str(cfunc) if cfunc else ""
        except Exception as exc:  # noqa: BLE001
            text = "hexrays failed: %s" % exc
        lines = text.splitlines()
        record["lines"] = len(lines)
        if len(lines) > MAX_LINES:
            lines = lines[:MAX_LINES] + ["... (%d more)" % (record["lines"] - MAX_LINES)]
        path = os.path.join(OUT_DIR, "%s_%x.c" % (label, ea))
        with open(path, "w", encoding="utf-8") as handle:
            handle.write("\n".join(lines) + "\n")
        record["file"] = path
        summary["targets"].append(record)
        with open(os.path.join(OUT_DIR, "summary.json"), "w", encoding="utf-8") as handle:
            json.dump(summary, handle, indent=2, ensure_ascii=False)
        print("[decompile] %s %#x lines=%d" % (label, ea, record["lines"]))

    print("wrote %s" % os.path.join(OUT_DIR, "summary.json"))


if __name__ == "__main__":
    main()

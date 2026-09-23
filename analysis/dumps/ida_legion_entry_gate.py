#!/usr/bin/env python3
"""Decompile the legion entry trigger points and one level of what they call.

Context: analysis/tasks/next64-legion-apocalypse-plan.md §6.2 (D1a) recorded that
`sub_142510A50` / `sub_142511D10` are the points the client uses to decide whether
the legion entry exists, and that they are where to look when a live client shows
no entry at all. That is now the live situation, so this pulls their pseudocode.

Written to be interruptible: every target is flushed to disk as soon as it is
decompiled, so a slow or stuck function cannot cost the whole run (the failure
mode that wasted a pass on the NOTI2657 probe).

    idat -A -L<log> -S"analysis/dumps/ida_legion_entry_gate.py" <work>.i64
"""

import json
import os
from datetime import datetime

import ida_funcs
import ida_hexrays
import idautils

OUT_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "legion-entry-gate")

# The two entry trigger points, plus the surrounding cluster so a caller/callee
# relationship is visible even if the name of one of them turns out to be wrong.
ANCHORS = [
    ("entry_142510a50", 0x142510A50),
    ("entry_142511d10", 0x142511D10),
]

INCLUDE_CALLEES = True
MAX_CALLEES = 12
MAX_LINES = 700


def main():
    os.makedirs(OUT_DIR, exist_ok=True)
    summary = {"generated": datetime.now().isoformat(timespec="seconds"), "targets": []}

    queue = list(ANCHORS)
    seen = {ea for _, ea in queue}
    if INCLUDE_CALLEES:
        added = 0
        for _, ea in ANCHORS:
            f = ida_funcs.get_func(ea)
            if f is None:
                continue
            for item in idautils.FuncItems(f.start_ea):
                for target in idautils.CodeRefsFrom(item, 0):
                    tf = ida_funcs.get_func(target)
                    if tf is None or tf.start_ea in seen:
                        continue
                    seen.add(tf.start_ea)
                    queue.append(("callee_%s" % ida_funcs.get_func_name(tf.start_ea).replace(".", "_"), tf.start_ea))
                    added += 1
                    if added >= MAX_CALLEES:
                        break
                if added >= MAX_CALLEES:
                    break

    for label, ea in queue:
        record = {"label": label, "ea": hex(ea), "func_name": ida_funcs.get_func_name(ea)}
        try:
            cfunc = ida_hexrays.decompile(ea)
            text = str(cfunc) if cfunc else ""
        except Exception as exc:  # noqa: BLE001 - report, never abort the batch
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
        # Flush the index after every function so an interrupted run still leaves
        # a usable index next to whatever .c files made it out.
        with open(os.path.join(OUT_DIR, "summary.json"), "w", encoding="utf-8") as handle:
            json.dump(summary, handle, indent=2, ensure_ascii=False)
        print("[decompile] %s %#x %s lines=%d" % (label, ea, record["func_name"], record["lines"]))

    print("wrote %s" % os.path.join(OUT_DIR, "summary.json"))


if __name__ == "__main__":
    main()

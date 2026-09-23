#!/usr/bin/env python3
"""Decompile the legion/apocalypse NOTI handlers and their callees (X9 / X11).

These are server-to-client packets, so the client has no *writer* to read: the
layout can only come from the **consumer**. Each handler reads a fixed number of
bytes off the packet cursor (next65 §2.2) and then hands them to a subsystem, so
the field order lives one level down.

Targets, all from analysis/tasks/next65-legion-packet-table.md:

    sub_1424FDDB0  NOTI2895  LEGION_INFO                      reads u16==107 + 204 B
    sub_1424FDF20  NOTI2896  LEGION_OPERATION                 reads u16==107 + 7 B
    sub_1424FDC30  NOTI2252  LEGION_BASIC_CLEAR_REWARD        reads 7772 B
    sub_1424FDB60  NOTI2253  LEGION_ADDITIONAL_CLEAR_REWARD   reads 2405 B
    sub_1424FDD50  NOTI2254  LEGION_ENTRY_CHARAC_INFO         reads 256 B
    sub_1424FE000  NOTI2568  PREPARE_LEGION_ENTER_DUNGEON     reads 16 B
    sub_1424FDF70  NOTI2657  LEGION_PHASE_CLEAR_TICK          reads 144 B (X7)

Written to be interruptible: every target is flushed as soon as it is decompiled,
so a slow function cannot cost the whole run.

    idat -A -L<log> -S"analysis/dumps/ida_legion_noti_consumers.py" <work>.i64
"""

import json
import os
from datetime import datetime

import ida_funcs
import ida_hexrays
import idautils

OUT_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "legion-noti-consumers")

ANCHORS = [
    ("noti2895_legion_info", 0x1424FDDB0),
    ("noti2896_legion_operation", 0x1424FDF20),
    ("noti2252_basic_reward", 0x1424FDC30),
    ("noti2253_additional_reward", 0x1424FDB60),
    ("noti2254_entry_charac_info", 0x1424FDD50),
    ("noti2568_prepare_enter", 0x1424FE000),
    ("noti2657_phase_tick", 0x1424FDF70),
]

INCLUDE_CALLEES = True
MAX_CALLEES = 60
MAX_LINES = 500


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
                print("[warn] no function at %#x" % ea)
                continue
            for item in idautils.FuncItems(f.start_ea):
                for target in idautils.CodeRefsFrom(item, 0):
                    tf = ida_funcs.get_func(target)
                    if tf is None or tf.start_ea in seen:
                        continue
                    seen.add(tf.start_ea)
                    name = ida_funcs.get_func_name(tf.start_ea).replace(".", "_")
                    queue.append(("callee_%s" % name, tf.start_ea))
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
        with open(os.path.join(OUT_DIR, "summary.json"), "w", encoding="utf-8") as handle:
            json.dump(summary, handle, indent=2, ensure_ascii=False)
        print("[decompile] %s %#x %s lines=%d" % (label, ea, record["func_name"], record["lines"]))

    print("wrote %s" % os.path.join(OUT_DIR, "summary.json"))


if __name__ == "__main__":
    main()

#!/usr/bin/env python3
"""Find who triggers the legion C2S senders (callers of sub_1424FE550 etc.),
so we know when the client actually enters the legion channel."""

import json
import os

import ida_funcs
import ida_hexrays
import idautils
import idc

SENDERS = {
    "2043_start": 0x1424FE550,
    "2044_fail": 0x1424FE340,
    "2045_enter_dungeon": 0x1424FE290,
    "2046_reward_end": 0x1424FE4A0,
    "2354_operation_select": 0x1424FE3F0,
    "2355_role_select": 0x14069E3B0,
}
OUT_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "sender-callers")


def owner(ea):
    f = ida_funcs.get_func(ea)
    if f is None:
        return None, None
    return ida_funcs.get_func_name(f.start_ea), f.start_ea


report = {"senders": [], "decompiled": []}
owners = {}
for label, va in SENDERS.items():
    entry = {"label": label, "va": hex(va), "callers": []}
    for xref in idautils.CodeRefsTo(va, 0):
        name, start = owner(xref)
        entry["callers"].append({"ea": hex(xref), "func": name, "func_ea": hex(start) if start else None})
        if start:
            owners[start] = name
    report["senders"].append(entry)

os.makedirs(OUT_DIR, exist_ok=True)
for start, name in sorted(owners.items()):
    label = "caller_%s_%x" % (name, start)
    item = {"label": label, "ea": hex(start)}
    try:
        cfunc = ida_hexrays.decompile(start)
        if cfunc is None:
            item["ok"] = False
        else:
            text = str(cfunc)
            item["ok"] = True
            item["lines"] = text.count("\n")
            with open(os.path.join(OUT_DIR, label + ".c"), "w", encoding="utf-8") as fh:
                fh.write(text)
    except Exception as exc:  # noqa: BLE001
        item["ok"] = False
        item["error"] = str(exc)
    report["decompiled"].append(item)

with open(os.path.join(OUT_DIR, "sender-callers.json"), "w", encoding="utf-8") as fh:
    json.dump(report, fh, indent=2, ensure_ascii=False)
print("[callers] senders=%d distinct callers=%d" % (len(SENDERS), len(owners)))
for entry in report["senders"]:
    print("  %-22s %s -> %s" % (entry["label"], entry["va"],
                                [c["func"] for c in entry["callers"]] or "NO XREF"))
idc.qexit(0)

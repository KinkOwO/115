#!/usr/bin/env python3
"""P3 evidence pass: the 256-byte entry-character struct's default builder,
its consumer, and who triggers APOCALYPSE_ROLE_SELECT (2355)."""

import json
import os

import ida_funcs
import ida_hexrays
import idautils
import idc

DECOMPILE = [
    ("init_entry_charac_1424fd050", 0x1424FD050),
    ("entry_charac_consumer_142abf5e0", 0x142ABF5E0),
    ("role_select_sender_14069e3b0", 0x14069E3B0),
]
CALLER_OF = {"role_select_sender": 0x14069E3B0}
OUT_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "p3-survey")


def owner(ea):
    f = ida_funcs.get_func(ea)
    if f is None:
        return None, None
    return ida_funcs.get_func_name(f.start_ea), f.start_ea


report = {"callers": [], "decompiled": []}
owners = {}
for label, va in CALLER_OF.items():
    for xref in idautils.CodeRefsTo(va, 0):
        name, start = owner(xref)
        entry = {"target": label, "ea": hex(xref), "func": name, "func_ea": hex(start) if start else None}
        report["callers"].append(entry)
        if start:
            owners[start] = name

os.makedirs(OUT_DIR, exist_ok=True)
for start, name in sorted(owners.items()):
    DECOMPILE.append(("caller_%s_%x" % (name or "unknown", start), start))

for label, ea in DECOMPILE:
    item = {"label": label, "ea": hex(ea)}
    try:
        cfunc = ida_hexrays.decompile(ea)
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

with open(os.path.join(OUT_DIR, "p3-survey.json"), "w", encoding="utf-8") as fh:
    json.dump(report, fh, indent=2, ensure_ascii=False)
print("[p3] callers=%d decompiled=%d" % (len(report["callers"]), len(report["decompiled"])))
for entry in report["callers"]:
    print("   %s %s -> %s" % (entry["target"], entry["ea"], entry["func"]))
idc.qexit(0)

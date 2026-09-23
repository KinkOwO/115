#!/usr/bin/env python3
"""Find the code that reads the apocalypse.ctp columns, by following the decoded
string globals the client compares against. Each tag below is a column name in
the table's schema; the function referencing it is the reader."""

import json
import os

import ida_funcs
import ida_hexrays
import idautils
import idc

TAGS = {
    "allow_coin": 0x14B2D7A20,
    "phase_info": 0x14B2D7B90,
    "gate_schedule": 0x14B2D7B30,
    "operation_data_set": 0x14B2D7960,
    "role_per_member_limit": 0x14B2D7928,
    "member_limit": 0x14B2D79F8,
    "gateflow": 0x14B2D7BB8,
    "gate_close_warning": 0x14B2D7B58,
    "recommend_fame": 0x14B26BC40,
    "monster_phase_info": 0x14B32A7F8,
    "reward_phase_info": 0x14B31A860,
    "skirmisher_info": 0x1492E0940,
    "gaurdian": 0x1492E0AA8,
}
OUT_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "ctp-readers")


def owner(ea):
    f = ida_funcs.get_func(ea)
    if f is None:
        return None, None
    return ida_funcs.get_func_name(f.start_ea), f.start_ea


report = {"tags": [], "decompiled": []}
owners = {}
for tag, va in TAGS.items():
    entry = {"tag": tag, "va": hex(va), "refs": []}
    for xref in idautils.DataRefsTo(va):
        name, start = owner(xref)
        entry["refs"].append({"ea": hex(xref), "func": name, "func_ea": hex(start) if start else None})
        if start:
            owners.setdefault(start, set()).add(tag)
    for xref in idautils.CodeRefsTo(va, 0):
        name, start = owner(xref)
        entry["refs"].append({"ea": hex(xref), "func": name, "func_ea": hex(start) if start else None, "code": True})
        if start:
            owners.setdefault(start, set()).add(tag)
    report["tags"].append(entry)

os.makedirs(OUT_DIR, exist_ok=True)
for start, tags in sorted(owners.items()):
    label = "reader_%x_%s" % (start, "_".join(sorted(tags))[:40])
    item = {"label": label, "ea": hex(start), "tags": sorted(tags)}
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

with open(os.path.join(OUT_DIR, "ctp-readers.json"), "w", encoding="utf-8") as fh:
    json.dump(report, fh, indent=2, ensure_ascii=False)
print("[ctp] tags=%d distinct reader funcs=%d" % (len(TAGS), len(owners)))
for start, tags in sorted(owners.items()):
    print("   %#x  %s" % (start, ", ".join(sorted(tags))))
idc.qexit(0)

#!/usr/bin/env python3
"""Who binds the S2C payload cursor (sub_146EA2160 / sub_146EA2170), and to which
buffer? Decides whether the server's S2C body includes the 13-byte envelope or not."""

import json
import os

import ida_funcs
import ida_hexrays
import idautils
import idc

BIND_PTR = 0x146EA2160
BIND_LEN = 0x146EA2170
OUT_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "cursor-bind-sites")


def owner(ea):
    f = ida_funcs.get_func(ea)
    if f is None:
        return None, None
    return ida_funcs.get_func_name(f.start_ea), f.start_ea


report = {"callers": [], "decompiled": []}
owners = {}
for label, va in (("bind_ptr", BIND_PTR), ("bind_len", BIND_LEN)):
    for xref in idautils.CodeRefsTo(va, 0):
        name, start = owner(xref)
        entry = {"target": label, "ea": hex(xref), "func": name, "func_ea": hex(start) if start else None}
        report["callers"].append(entry)
        if start:
            owners[start] = name

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

with open(os.path.join(OUT_DIR, "bind-sites.json"), "w", encoding="utf-8") as fh:
    json.dump(report, fh, indent=2, ensure_ascii=False)
print("[bind] callers=%d distinct funcs=%d" % (len(report["callers"]), len(owners)))
for entry in report["callers"]:
    print("   %-9s %s  %s" % (entry["target"], entry["ea"], entry["func"]))
idc.qexit(0)

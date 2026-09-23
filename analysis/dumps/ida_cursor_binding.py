#!/usr/bin/env python3
"""Bind the S2C payload cursor: who writes qword_14F1BF870 / dword_14F1BF878,
and what sub_146EA0340 does. Answering this turns the per-handler read sizes
into a hard server-side contract."""

import json
import os

import ida_funcs
import ida_hexrays
import idaapi
import idautils
import idc

CURSOR_PTR = 0x14F1BF870
CURSOR_LEN = 0x14F1BF878
CONSUME = 0x146EA0340
OUT_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "cursor-binding")


def func_name(ea):
    f = ida_funcs.get_func(ea)
    if f is None:
        return None, None
    return ida_funcs.get_func_name(f.start_ea), f.start_ea


report = {"cursor_ptr": hex(CURSOR_PTR), "cursor_len": hex(CURSOR_LEN), "writers": [], "consumers": []}

# Who references the cursor globals, per data-xref direction.
for label, va, bucket in (("cursor_ptr", CURSOR_PTR, "writers"), ("cursor_len", CURSOR_LEN, "writers")):
    sites = []
    for xref in idautils.DataRefsTo(va):
        name, start = func_name(xref)
        sites.append({"ea": hex(xref), "func": name, "func_ea": hex(start) if start else None})
    report[bucket].append({"global": label, "va": hex(va), "refs": sites})

# Consumers of the read API (called right after every read).
for xref in idautils.DataRefsTo(CONSUME):
    name, start = func_name(xref)
    report["consumers"].append({"ea": hex(xref), "func": name, "func_ea": hex(start) if start else None})
for xref in idautils.CodeRefsTo(CONSUME, 0):
    name, start = func_name(xref)
    report["consumers"].append({"ea": hex(xref), "func": name, "func_ea": hex(start) if start else None})

# Decompile the consume helper and every distinct writer function.
targets = {"consume_146ea0340": CONSUME}
for entry in report["writers"]:
    for ref in entry["refs"]:
        if ref["func_ea"]:
            targets["writer_" + (ref["func"] or "unknown") + "_" + ref["func_ea"][2:]] = int(ref["func_ea"], 16)

report["decompiled"] = []
os.makedirs(OUT_DIR, exist_ok=True)
for label, ea in targets.items():
    entry = {"label": label, "ea": hex(ea)}
    try:
        cfunc = ida_hexrays.decompile(ea)
        if cfunc is None:
            entry["ok"] = False
        else:
            text = str(cfunc)
            entry["ok"] = True
            entry["lines"] = text.count("\n")
            with open(os.path.join(OUT_DIR, label + ".c"), "w", encoding="utf-8") as fh:
                fh.write(text)
    except Exception as exc:  # noqa: BLE001 - survey script, report anything
        entry["ok"] = False
        entry["error"] = str(exc)
    report["decompiled"].append(entry)

with open(os.path.join(OUT_DIR, "cursor-binding.json"), "w", encoding="utf-8") as fh:
    json.dump(report, fh, indent=2, ensure_ascii=False)
print("[cursor] ptr refs=%d len refs=%d consumers=%d decompiled=%d" % (
    len(report["writers"][0]["refs"]), len(report["writers"][1]["refs"]),
    len(report["consumers"]), len(report["decompiled"])))
print("[cursor] wrote cursor-binding.json")
idc.qexit(0)

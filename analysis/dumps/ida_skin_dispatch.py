#!/usr/bin/env python3
"""Decompile the shared dispatch candidates that reference most skin opcodes.

sub_1457A7110 references 7 of the 8 skin ids -> likely the central NOTI/CMD
dispatch switch. Dump it (and the other multi-id functions) so the per-opcode
handler for each skin packet can be read straight from the switch arms.

Run:  idat -A -L<log> -S"ida_skin_dispatch.py" <work>/DFO.exe.i64
"""
import os
import ida_hexrays
import ida_funcs
import idc

OUT_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "skin-noti")
VAS = [0x1457A7110, 0x1444E81C0, 0x14567C390, 0x14782B210, 0x146D0BDD0, 0x1476E0120]

os.makedirs(OUT_DIR, exist_ok=True)
for ea in VAS:
    f = ida_funcs.get_func(ea)
    start = f.start_ea if f else ea
    name = ida_funcs.get_func_name(start) or ("sub_%X" % start)
    try:
        cf = ida_hexrays.decompile(start)
    except Exception as exc:
        print("FAIL %x %r" % (start, exc))
        continue
    if cf is None:
        print("NONE %x" % start)
        continue
    text = str(cf)
    path = os.path.join(OUT_DIR, "dispatch_%s.c" % name)
    with open(path, "w", encoding="utf-8") as fh:
        fh.write(text)
    print("OK %x %-24s lines=%d -> %s" % (start, name, text.count("\n"), path))
idc.qexit(0)

#!/usr/bin/env python3
"""Decompile sub_1444EE4F0 (3rd caller of the damage-font name mapper) and the
cargo-reset helper sub_1444F0A40, plus scan sub_1444EE4F0 for page-tree offset
immediates (136+16*p) and lookup_page_id calls, to pin the damage-font page."""
import os
import ida_hexrays
import ida_funcs
import ida_name
import idaapi
import idc

OUT_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "skin-noti")
os.makedirs(OUT_DIR, exist_ok=True)


def dump(fea, tag):
    nm = ida_name.get_name(fea) or "sub_%X" % fea
    try:
        cf = ida_hexrays.decompile(fea)
    except Exception as exc:
        print("FAIL %s %s %r" % (tag, nm, exc)); return None
    if cf is None:
        print("NONE %s %s" % (tag, nm)); return None
    text = str(cf)
    p = os.path.join(OUT_DIR, "df9_%s_%s.c" % (tag, ("%X" % fea).lower()))
    open(p, "w", encoding="utf-8").write(text)
    print("OK %s %s lines=%d -> %s" % (tag, nm, text.count("\n"), p))
    return text


dump(0x1444EE4F0, "df_page_render")
dump(0x1444F0A40, "cargo_reset2")

# Scan sub_1444EE4F0 disasm for page-tree offsets and lookup calls
PAGE_OFF = {136 + 16 * p: p for p in range(10)}
LOOKUP = 0x1444EBD90
NAME_MAP = 0x1444EC790
f = ida_funcs.get_func(0x1444EE4F0)
print("=== scan sub_1444EE4F0 (0x%X-0x%X) ===" % (f.start_ea, f.end_ea))
ea = f.start_ea
while ea < f.end_ea:
    m = idc.print_insn_mnem(ea)
    dis = idc.generate_disasm_line(ea, 0)
    for op in (0, 1, 2):
        if idc.get_operand_type(ea, op) == idc.o_displ:
            d = idc.get_operand_value(ea, op)
            if d in PAGE_OFF:
                print("PAGE-TREE off %d (page %d) @ %X: %s" % (d, PAGE_OFF[d], ea, dis))
        if idc.get_operand_type(ea, op) == idc.o_imm:
            v = idc.get_operand_value(ea, op)
            if v in PAGE_OFF:
                print("PAGE-IMM %d (page %d) @ %X: %s" % (v, PAGE_OFF[v], ea, dis))
    if m == "call":
        t = idc.get_operand_value(ea, 0)
        if t == LOOKUP:
            print("CALL lookup_page_id @ %X: %s" % (ea, dis))
        elif t == NAME_MAP:
            print("CALL df_name_map @ %X: %s" % (ea, dis))
    ea = idc.next_head(ea, f.end_ea)

idc.qexit(0)

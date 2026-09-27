#!/usr/bin/env python3
"""Confirm cargo page 4 == damage font by decompiling the page-4-specific code
paths and the cargo tab caption setup.

NOTI1546 reader case 4 (page 4) calls: sub_14603E2A0(id,0) -> sub_14057E020 ->
  sub_140422B30.  General-select case 4 calls sub_145D87890 / sub_145D1D810 on
  the sub_145EFAFB0 context.  If these resolve to damage-font effect/name
  resources, page 4 is the damage-font category.

Also: locate the cargo window-730 tab caption setup (page index -> dstr label)
by scanning callers of sub_1441ECDB0 (page select) for caption/string calls.
"""
import os
import ida_hexrays
import ida_funcs
import ida_name
import ida_xref
import idaapi
import idc

OUT_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "skin-noti")
os.makedirs(OUT_DIR, exist_ok=True)


def dump(fea, tag):
    if not fea:
        return
    nm = ida_name.get_name(fea) or "sub_%X" % fea
    try:
        cf = ida_hexrays.decompile(fea)
    except Exception as exc:
        print("FAIL %s %s %r" % (tag, nm, exc)); return
    if cf is None:
        print("NONE %s %s" % (tag, nm)); return
    text = str(cf)
    p = os.path.join(OUT_DIR, "df10_%s_%s.c" % (tag, ("%X" % fea).lower()))
    open(p, "w", encoding="utf-8").write(text)
    print("OK %s %s lines=%d -> %s" % (tag, nm, text.count("\n"), p))


for tag, fea in (("p4_noti1546_a", 0x14603E2A0),
                 ("p4_noti1546_b", 0x14057E020),
                 ("p4_noti1546_c", 0x140422B30),
                 ("p4_select_a", 0x145D87890),
                 ("p4_select_b", 0x145D1D810),
                 ("ctx_getter", 0x145EFAFB0)):
    dump(fea, tag)

# Scan the cargo page-select callers for caption/label string calls
print("=== cargo tab controllers (callers of sub_1441ECDB0) ===")
for fea in (0x1441D6570, 0x1441D8460, 0x1441DB060):
    f = ida_funcs.get_func(fea)
    if not f:
        continue
    print("--- controller 0x%X size %d ---" % (fea, f.end_ea - f.start_ea))
    ea = f.start_ea
    cnt = 0
    while ea < f.end_ea and cnt < 4000:
        cnt += 1
        if idc.print_insn_mnem(ea) == "call":
            t = idc.get_operand_value(ea, 0)
            tn = ida_name.get_name(t) or ""
            # caption/text setters often named; flag any call whose target name
            # hints at text, or any lea of a .rdata string just before
            if "1441ECDB0" in ("%X" % t) or t == 0x1441ECDB0:
                # find preceding page-index immediate
                pe = ea
                for _ in range(6):
                    pe = idc.prev_head(pe, pe - 0x30)
                    if pe == idaapi.BADADDR:
                        break
                    if idc.print_insn_mnem(pe) == "mov" and idc.get_operand_type(pe, 1) == idc.o_imm:
                        print("  page-select idx=%d @ %X" % (idc.get_operand_value(pe, 1), ea))
                        break
        ea = idc.next_head(ea, f.end_ea)

idc.qexit(0)

#!/usr/bin/env python3
"""Pin the damage-font cargo PAGE index from true source.

Two independent angles:

A) FULLY decompile the cargo tab controllers (callers of the page-select
   sub_1441ECDB0) and the page-4 preview chain (sub_14603E2A0 ->
   sub_14603E980 -> sub_14603E2D0).  If the page-4 preview singleton is the
   damage-font effect manager, page 4 == damage fonts.

B) Robust global xref sweep: every caller of the dstr->wstring converter
   sub_14723C170 that passes a *damage-font* dstr id (19666 "Damage Font",
   100002264 "You have no Damage fonts.", ...).  The function that renders the
   "Damage Font" tab / empty-list caption must read one specific cargo page
   tree (mgr+136+16*p).  Decompile those functions too.
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

DSTR_CONV = 0x14723C170          # dstr id -> wstring
UI_SETTEXT = 0x14668C520         # set UI element text

# damage-font related dstr ids (from analysis/dumps/dstr_raw.txt)
DF_IDS = {
    19662: "Changed the Damage font.",
    19666: "Damage Font",
    19667: "It's your current Damage font.",
    100002106: "damage font only in town / translucency anywhere",
    100002108: "Changed damage font.",
    100002109: "Changed damage font and translucency.",
    100002110: "only change damage font while in town",
    100002111: "Changed damage translucency / font only in town",
    100002264: "You have no Damage fonts.",
    101035806: "Show Special Damage font",
    101035809: "Show Party Member Special Damage font",
}


def dump(fea, tag):
    if not fea:
        return None
    nm = ida_name.get_name(fea) or "sub_%X" % fea
    try:
        cf = ida_hexrays.decompile(fea)
    except Exception as exc:
        print("FAIL %s %s %r" % (tag, nm, exc))
        return None
    if cf is None:
        print("NONE %s %s" % (tag, nm))
        return None
    text = str(cf)
    p = os.path.join(OUT_DIR, "df12_%s_%s.c" % (tag, ("%X" % fea).lower()))
    open(p, "w", encoding="utf-8").write(text)
    print("OK %s %s lines=%d -> %s" % (tag, nm, text.count("\n"), p))
    return text


print("=== A) controllers + page-4 preview chain (full decompile) ===")
for tag, fea in (("ctrl_a", 0x1441D6570),
                 ("ctrl_b", 0x1441D8460),
                 ("ctrl_c", 0x1441DB060),
                 ("page_select", 0x1441ECDB0),
                 ("p4_preview_get", 0x14603E980),
                 ("p4_preview_apply", 0x14603E2D0),
                 ("p4_preview_wrap", 0x14603E2A0)):
    dump(fea, tag)


def imm_args_into(func_ea, target):
    """Return list of (call_ea, imm) where an imm is set on a register shortly
    before a `call target` inside func [heuristic: scan all imm operands whose
    value is a damage-font dstr id]."""
    hits = []
    f = ida_funcs.get_func(func_ea)
    if not f:
        return hits
    ea = f.start_ea
    n = 0
    while ea < f.end_ea and n < 20000:
        n += 1
        mnem = idc.print_insn_mnem(ea)
        if mnem in ("mov", "push", "lea", "cmp"):
            for op in (0, 1):
                if idc.get_operand_type(ea, op) == idc.o_imm:
                    v = idc.get_operand_value(ea, op)
                    if v in DF_IDS:
                        hits.append((ea, v))
        ea = idc.next_head(ea, f.end_ea)
    return hits


print("")
print("=== B) global sweep: callers of dstr converter passing a damage-font id ===")
import idautils
seen_funcs = {}
xrefs = [xr.frm for xr in idautils.XrefsTo(DSTR_CONV, 0)]
print("total xrefs to dstr converter: %d" % len(xrefs))
for frm in xrefs:
    f = ida_funcs.get_func(frm)
    if not f:
        continue
    fs = f.start_ea
    if fs in seen_funcs:
        continue
    hits = imm_args_into(fs, DSTR_CONV)
    if hits:
        seen_funcs[fs] = hits
        nm = ida_name.get_name(fs) or "sub_%X" % fs
        for (ea, v) in hits:
            print("  func 0x%X (%s) imm=%d (%s) @ 0x%X" % (fs, nm, v, DF_IDS[v], ea))

print("")
print("=== decompiling damage-font-referencing functions ===")
for fs in list(seen_funcs.keys()):
    dump(fs, "dfref")

idc.qexit(0)

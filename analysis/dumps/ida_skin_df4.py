#!/usr/bin/env python3
"""Confirm skin-identity semantics and locate the damage-font dispatch.

1) Decompile sub_1473A1120 — the id mapper used by BOTH the NOTI1545 cargo reader
   (count2 loop) and the CMD1565 SELECT_SKIN handler. Reveals what a "skin id" is.
2) The damage-font packets (noti1231 / cmd1282) are not in the literal-id registrar.
   Find them via xrefs to their enum-name string VAs from opcodes.tsv:
     noti1231 DAMAGE_FONT_SKIN_LIST   string 0x14b010440  table 0x14ef35b28
     cmd1282  SELECT_DAMAGE_FONT_SKIN string 0x14b05a790  table 0x14ef3b770
     cmd857   OPEN_AURA_SKIN_SLOT     string 0x14b050fd0  table 0x14ef3aa28
   Decompile every function that references those VAs.
3) Decompile the cargo-tree consumers: the NOTI1545 reader inserts nodes keyed by
   the low u32 of each u64 entry; dump sub_1444E8030 (the insert) to see the node
   layout, and sub_1444F1EE0 (called at the end of 1546/1565) to see the render.

Run:  idat -A -L<log> -S"ida_skin_df4.py" <work>/DFO.exe.i64
"""
import os
import ida_funcs
import ida_hexrays
import ida_xref
import idaapi
import idc

OUT_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "skin-noti")
os.makedirs(OUT_DIR, exist_ok=True)

DIRECT = {
    0x1473A1120: "mapper_skin_id",
    0x1444E8030: "cargo_tree_insert",
    0x1444F1EE0: "cargo_render_tail",
    0x1444EC790: "cmd1565_branch_a2_0",
}

NAME_VAS = {
    0x14b010440: "noti1231_damage_font_skin_list_str",
    0x14ef35b28: "noti1231_damage_font_skin_list_tbl",
    0x14b05a790: "cmd1282_select_damage_font_skin_str",
    0x14ef3b770: "cmd1282_select_damage_font_skin_tbl",
    0x14b050fd0: "cmd857_open_aura_skin_slot_str",
    0x14ef3aa28: "cmd857_open_aura_skin_slot_tbl",
}


def decomp(ea):
    f = ida_funcs.get_func(ea)
    start = f.start_ea if f else ea
    try:
        cf = ida_hexrays.decompile(start)
    except Exception as exc:
        return None, start, repr(exc)
    return (str(cf) if cf is not None else None), start, None


def dump(ea, label):
    text, start, err = decomp(ea)
    if text is None:
        print("FAIL %s %x %s" % (label, start, err))
        return
    p = os.path.join(OUT_DIR, "df4_%s_%x.c" % (label, start))
    open(p, "w", encoding="utf-8").write(text)
    print("OK %s %x lines=%d -> %s" % (label, start, text.count("\n"), p))


for ea, label in DIRECT.items():
    dump(ea, label)

for va, label in NAME_VAS.items():
    refs = []
    xb = ida_xref.get_first_cref_to(va)
    while xb != idaapi.BADADDR:
        refs.append(xb)
        xb = ida_xref.get_next_cref_to(va, xb)
    # also data refs
    db = ida_xref.get_first_dref_to(va)
    while db != idaapi.BADADDR:
        refs.append(db)
        db = ida_xref.get_next_dref_to(va, db)
    print("XREF %s 0x%x count=%d %s" % (label, va, len(refs), ["0x%X" % r for r in refs[:12]]))
    seen = set()
    for r in refs:
        f = ida_funcs.get_func(r)
        fea = f.start_ea if f else 0
        if fea and fea not in seen:
            seen.add(fea)
            dump(fea, "%s_user" % label)

idc.qexit(0)

#!/usr/bin/env python3
"""Decompile the delegated body readers behind the skin-cargo NOTIs.

  noti1545 SKIN_CARGO_INFO   handler sub_1444ED900 reads u8 page then calls
                             sub_1444EFF40(mgr, page)  <- real cargo body reader
  noti1546 SELECT_SKIN_LIST  handler sub_1444ED4F0 reads u8 page then calls
                             sub_1444EECA0(mgr, page)  <- real select-list reader
  noti1547 RECENT_ADD_SKIN_LIST is fully inline already (u8 count + {u8,u32}*n).

Also decompile the consumers of the 1547 vector at mgr bytes 1096/1104/1112
to learn what the {u8 flag, u32 value} entry means (skin index vs template).

Run:  idat -A -L<log> -S"ida_skin_readers.py" <work>/DFO.exe.i64
"""
import os
import ida_funcs
import ida_hexrays
import idc

OUT_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "skin-noti")
os.makedirs(OUT_DIR, exist_ok=True)

FUNCS = {
    0x1444EFF40: "reader_1545_cargo_body",
    0x1444EECA0: "reader_1546_select_body",
    0x1444F0270: "reader_1671_tag_page",
    0x1444EFA00: "reader_1672_tag_entry",
    0x1444EC790: "cmd1565_select_skin_a3",
    0x1444EE820: "cmd1565_select_skin_else",
}


def decomp(ea):
    f = ida_funcs.get_func(ea)
    start = f.start_ea if f else ea
    try:
        cf = ida_hexrays.decompile(start)
    except Exception as exc:
        return None, start, repr(exc)
    if cf is None:
        return None, start, "none"
    return str(cf), start, None


for ea, label in FUNCS.items():
    text, start, err = decomp(ea)
    if text is None:
        print("FAIL %s %x %s" % (label, start, err))
        continue
    p = os.path.join(OUT_DIR, "rd_%s_%x.c" % (label, start))
    open(p, "w", encoding="utf-8").write(text)
    print("OK %s %x lines=%d -> %s" % (label, start, text.count("\n"), p))

idc.qexit(0)

#!/usr/bin/env python3
"""Decompile the skin-cargo NOTI/CMD handlers found in the manager constructor
sub_1444E81C0, which registers them via sub_14599D5D0(mgr, <decimal id>, handler, 0).

  1545 SKIN_CARGO_INFO       -> sub_1444ED900
  1546 SELECT_SKIN_LIST      -> sub_1444ED4F0
  1547 RECENT_ADD_SKIN_LIST  -> sub_1444ED400
  1565 SELECT_SKIN (cmd)     -> sub_1444E8D00
  1671 TAG_TOURNAMENT_SKIN_CARGO_INFO -> sub_1444EE1C0
  1672 TAG_TOURNAMENT_SELECT_SKIN_LIST-> sub_1444EDFD0

The handler body's packet-reader call sequence is the authoritative S2C layout.

Run:  idat -A -L<log> -S"ida_skin_handler_bodies.py" <work>/DFO.exe.i64
"""
import os
import ida_hexrays
import ida_funcs
import idc

OUT_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "skin-noti")
HANDLERS = {
    0x1444ED900: "noti1545_skin_cargo_info",
    0x1444ED4F0: "noti1546_select_skin_list",
    0x1444ED400: "noti1547_recent_add_skin_list",
    0x1444E8D00: "cmd1565_select_skin",
    0x1444EE1C0: "noti1671_tag_skin_cargo_info",
    0x1444EDFD0: "noti1672_tag_select_skin_list",
}

os.makedirs(OUT_DIR, exist_ok=True)
for ea, label in HANDLERS.items():
    f = ida_funcs.get_func(ea)
    start = f.start_ea if f else ea
    try:
        cf = ida_hexrays.decompile(start)
    except Exception as exc:
        print("FAIL %s %x %r" % (label, start, exc))
        continue
    if cf is None:
        print("NONE %s %x" % (label, start))
        continue
    text = str(cf)
    path = os.path.join(OUT_DIR, "handler_%s_%x.c" % (label, start))
    with open(path, "w", encoding="utf-8") as fh:
        fh.write(text)
    print("OK %s %x lines=%d -> %s" % (label, start, text.count("\n"), path))
idc.qexit(0)

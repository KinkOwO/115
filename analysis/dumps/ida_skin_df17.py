#!/usr/bin/env python3
"""df17: read the skin-cargo panel table that holds the damage-font tab.

df16 closed two links:
  * `sub_1441ECDB0(win, tab)` asks the manager for page `tab` verbatim
    (`sub_1444EBC10(mgr, vec, tab)`), so UI tab index == cargo page index 0..9.
  * the damage-font panel builder `sub_1441CBDD0` (dstr 100002264 "You have no
    Damage fonts.") has exactly one xref: a function-pointer slot at
    0x14A230DF8. So the panels are an array and that array index is the tab.

Dump the array around 0x14A230DF8, name every neighbour, and show who reads the
table base so the index -> tab correspondence is read from the binary, not
guessed. Also re-decompile the page-vector reader with a working Hex-Rays call.
"""
import ida_bytes
import ida_funcs
import ida_hexrays
import ida_name
import idautils
import idc

TABLE = 0x14A230DF8
DF_PANEL = 0x1441CBDD0

print("=== function-pointer array around 0x%X ===" % TABLE)
start = TABLE - 16 * 8
for i in range(24):
    ea = start + 8 * i
    v = ida_bytes.get_qword(ea)
    f = ida_funcs.get_func(v) if v else None
    nm = ida_name.get_name(v) if v else ""
    mark = "  <== damage-font panel" if v == DF_PANEL else ""
    print("  slot %2d 0x%X -> 0x%-12X %-28s %s%s" % (i, ea, v, nm or "?",
                                                     "size=%d" % (f.end_ea - f.start_ea) if f else "", mark))

print("\n=== xrefs into the array neighbourhood (who indexes it) ===")
for i in range(-4, 20):
    ea = TABLE + 8 * i
    for x in idautils.XrefsTo(ea, 0):
        f = ida_funcs.get_func(x.frm)
        print("  0x%X <- 0x%X %s : %s" % (ea, x.frm, ida_name.get_name(f.start_ea) if f else "?",
                                          (idc.generate_disasm_line(x.frm, 0) or "").split("\n")[0]))

print("\n=== decompiles ===")
for ea, tag in ((0x1444EBC10, "page_ids(mgr,out,page)"), (0x1444EBAB0, "entry_by_id(id)"),
                (0x1441D8460, "cargo_window_refresh_all")):
    print("\n---- %s 0x%X ----" % (tag, ea))
    try:
        print(ida_hexrays.decompile(ea))
    except Exception as exc:
        print("  <failed %s>" % exc)

idc.qexit(0)

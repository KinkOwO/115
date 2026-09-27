#!/usr/bin/env python3
"""df20: pin the owned-page <-> selection-category model of the skin manager.

Evidence so far: the damage-font panel (class vtable off_14A230DE8 at window+9760)
reads its selection through sub_1444EBC10(mgr, out, 6), and the NOTI1546 body
sub_1444EECA0 case 6 checks ownership with sub_1444EBD90(mgr, 2, id) -- the same
page argument case 2 uses. So the tab/argument space is not a single index and the
two containers have to be separated before any cargo packet is emitted.

df20 therefore decompiles the accessors themselves (which container does page
argument k touch, what writes qword_14E683BF8, how NOTI1545 mode fills a page) and
the panel readers, instead of more call sites.
"""
import ida_bytes
import ida_funcs
import ida_hexrays
import ida_name
import idautils
import idc

TARGETS = (
    (0x1444EBD90, "mgr_page_lookup(mgr,k,id)"),
    (0x1444EBC10, "mgr_selection_read(mgr,out,cat)"),
    (0x1444EBAB0, "mgr_writer_1444EBAB0"),
    (0x1444EC2C0, "entry_expired_or_hidden"),
    (0x1444EFF40, "noti1545_mode_reader"),
    (0x1441C4080, "tab_dispatch"),
    (0x1441D21E0, "df_panel_refresh_a"),
    (0x1441D23A0, "df_panel_refresh_b"),
)
for ea, tag in TARGETS:
    print("\n---- %s 0x%X ----" % (tag, ea))
    try:
        print(ida_hexrays.decompile(ea))
    except Exception as exc:
        print("  <failed %s>" % exc)

print("\n=== xrefs to qword_14E683BF8 ===")
for xr in idautils.XrefsTo(0x14E683BF8):
    fn = ida_funcs.get_func(xr.frm)
    print("  0x%X %-14s type=%s in %s" % (
        xr.frm, idc.print_insn_mnem(xr.frm) or "?", xr.type,
        ida_name.get_name(fn.start_ea) if fn else "?"))

print("\n=== instructions writing qword_14E683BF8 (text scan of the xref funcs) ===")
seen = set()
for xr in idautils.XrefsTo(0x14E683BF8):
    fn = ida_funcs.get_func(xr.frm)
    if not fn or fn.start_ea in seen:
        continue
    seen.add(fn.start_ea)
    for head in idautils.FuncItems(fn.start_ea):
        txt = (idc.generate_disasm_line(head, 0) or "").split("\n")[0]
        if "E683BF8" in txt and txt.startswith(("mov", "call", "lea", "cmp", "test")):
            print("  0x%X %-20s : %s" % (head, ida_name.get_name(fn.start_ea), txt))

print("\n=== callers of noti1545_mode_reader 0x1444EFF40 ===")
for xr in idautils.XrefsTo(0x1444EFF40, 0):
    fn = ida_funcs.get_func(xr.frm)
    print("  0x%X in %s" % (xr.frm, ida_name.get_name(fn.start_ea) if fn else "?"))

print("\n=== dstr names used inside the damage-font panel refreshers ===")
for ea in (0x1441D21E0, 0x1441D23A0, 0x1441CBDD0):
    fn = ida_funcs.get_func(ea)
    if not fn:
        continue
    print("  ---- %s" % ida_name.get_name(fn.start_ea))
    for head in idautils.FuncItems(fn.start_ea):
        for xr in idautils.XrefsTo(head, 0):
            name = ida_name.get_name(xr.to) or ""
            if name.startswith(("aDstr", "dstr", "qword_14A", "off_")):
                print("    0x%X -> %s" % (head, name))

idc.qexit(0)

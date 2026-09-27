#!/usr/bin/env python3
"""df21: find what actually fills the damage-font grid (page 2 vs category 6).

df20 closed the containers:
  sub_1444EBD90(mgr,k,id)   -> map at mgr+136+16*k, keyed by id, payload{+8 expiry1,
                               +12 flag1, +16 expiry2, +20 flag2}; NOTI1545 mode k
                               clears exactly that map and refills it.
  sub_1444EBC10(mgr,out,cat)-> id vector of map at mgr+296 keyed by cat (NOTI1546).
  qword_14E683BF8 is a lazily constructed global singleton (single writer
  0x145A24DBC); every other xref only loads it, so it is not a cargo store.

Remaining gap: the panel refreshers use selection category 6, which NOTI1546 fills
with a single u32, while a grid needs the whole owned set. df21 therefore dumps the
panel builders to files and reports which container each one walks.
"""
import os
import ida_bytes
import ida_funcs
import ida_hexrays
import ida_name
import idautils
import idc

OUT = r"D:\115us\analysis\dumps\skin-noti"

TARGETS = (
    (0x1441CBDD0, "df_panel_builder"),
    (0x1441D8460, "window_refresh_all"),
    (0x1441ECDB0, "window_tab_refresh"),
    (0x1441E0820, "cell_factory"),
    (0x1441D6570, "df_panel_caller_a"),
    (0x1444F1410, "mgr_helper_f1410"),
    (0x1444E9630, "mgr_helper_e9630"),
    (0x1444F1920, "mgr_helper_f1920"),
    (0x1444EE820, "mgr_helper_ee820"),
    (0x1444EBEB0, "mgr_helper_ebeb0"),
    (0x1444EC1B0, "mgr_helper_ec1b0"),
    (0x1444EC0D0, "mgr_helper_ec0d0"),
)

index = []
for ea, tag in TARGETS:
    try:
        body = str(ida_hexrays.decompile(ea))
    except Exception as exc:
        index.append("%-22s 0x%X FAILED %s" % (tag, ea, exc))
        continue
    fn = ida_funcs.get_func(ea)
    name = ida_name.get_name(fn.start_ea) if fn else "?"
    path = os.path.join(OUT, "df21_%s.c" % tag)
    with open(path, "w", encoding="utf-8") as handle:
        handle.write("// %s = %s 0x%X\n\n%s\n" % (tag, name, ea, body))
    index.append("%-22s 0x%X %-18s -> df21_%s.c lines=%d" % (tag, ea, name, tag, body.count("\n")))

print("\n=== df21 dump index ===")
for line in index:
    print("  " + line)

print("\n=== which container does each panel function touch? ===")
import re
CALLS = ((0x1444EBC10, "selection_cat"), (0x1444EBD90, "owned_page"),
         (0x1444EC5B0, "sel_contains"), (0x1444EC2C0, "expiry_check"),
         (0x1444EFF40, "noti1545_fill"), (0x1441E0820, "cell_factory"))
for ea, tag in TARGETS + ((0x1441D21E0, "df_panel_refresh_a"), (0x1441D23A0, "df_panel_refresh_b")):
    fn = ida_funcs.get_func(ea)
    body = ""
    if fn:
        try:
            body = str(ida_hexrays.decompile(fn.start_ea))
        except Exception:
            pass
    hits = []
    for callee, label in CALLS:
        nm = ida_name.get_name(callee) or ""
        if nm and re.search(re.escape(nm) + r"\s*\(", body):
            args = re.findall(re.escape(nm) + r"\([^)]{0,60}\)", body)
            hits.append("%s%s" % (label, args[:3]))
    print("  %-22s %s" % (tag, " | ".join(hits) or "-"))

print("\n=== manager-range functions that index the page array (base+136+16*k) ===")
for f in idautils.Functions(0x1444E8000, 0x1444F2600):
    try:
        body = str(ida_hexrays.decompile(f))
    except Exception:
        continue
    if " 136" in body and "16LL *" in body:
        print("  0x%X %s" % (f, ida_name.get_name(f)))

print("\n=== dstr/skin-list enumerators used by the damage-font panel ===")
for ea in (0x1441CBDD0, 0x1441D6570):
    fn = ida_funcs.get_func(ea)
    if not fn:
        continue
    print("  ---- %s" % ida_name.get_name(fn.start_ea))
    for head in idautils.FuncItems(fn.start_ea):
        for xr in idautils.XrefsTo(head, 0):
            nm = ida_name.get_name(xr.to) or ""
            if nm.startswith(("aDstr", "dstr", "unk_14A", "off_14A")):
                print("    0x%X %s -> %s" % (head, idc.print_insn_mnem(head), nm))

idc.qexit(0)

#!/usr/bin/env python3
"""Final pass: pin the damage-font cargo PAGE index and id semantics.

Targets:
 1. sub_1444E92E0  - cargo setup called in ctor right before handler registration
                     (likely builds the page->category map / tab list).
 2. xrefs to sub_1444EC790 (damage-font name mapper) - every caller, to see how
    the damage-font category/page is referenced.
 3. sub_1444EBD90  - lookup(mgr, page, id) used by the 1546 reader.
 4. sub_1444E8ED0  - select/equip helper.
 5. sub_1444E8DF0  - insert helper used by NOTI2641 favorite rebuild.
 6. sub_1444EC2C0  - 'owned?' predicate.
 7. The cargo window-730 tab builder: functions that call sub_1441ECDB0 (page
    select) - decompile the smallest few to read page->label constants.
"""
import os
import ida_funcs
import ida_hexrays
import ida_xref
import ida_name
import idaapi
import idc

OUT_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "skin-noti")
os.makedirs(OUT_DIR, exist_ok=True)


def dump(fea, tag):
    if fea == idaapi.BADADDR or fea == 0:
        print("SKIP %s (bad)" % tag)
        return
    nm = ida_name.get_name(fea) or "sub_%X" % fea
    try:
        cf = ida_hexrays.decompile(fea)
    except Exception as exc:
        print("FAIL %s %s %r" % (tag, nm, exc)); return
    if cf is None:
        print("NONE %s %s" % (tag, nm)); return
    text = str(cf)
    p = os.path.join(OUT_DIR, "df8_%s_%s.c" % (tag, ("%X" % fea).lower()))
    open(p, "w", encoding="utf-8").write(text)
    print("OK %s %s lines=%d -> %s" % (tag, nm, text.count("\n"), p))


for tag, fea in (("cargo_setup", 0x1444E92E0),
                 ("df_name_map", 0x1444EC790),
                 ("lookup_page_id", 0x1444EBD90),
                 ("select_helper", 0x1444E8ED0),
                 ("insert_helper", 0x1444E8DF0),
                 ("owned_pred", 0x1444EC2C0)):
    dump(fea, tag)

# xrefs to the damage-font name mapper
print("=== callers of sub_1444EC790 (damage-font name mapper) ===")
DM = 0x1444EC790
for getter, nxt in ((ida_xref.get_first_cref_to, ida_xref.get_next_cref_to),
                    (ida_xref.get_first_dref_to, ida_xref.get_next_dref_to)):
    r = getter(DM)
    while r != idaapi.BADADDR:
        f = ida_funcs.get_func(r)
        fs = f.start_ea if f else 0
        print("caller func 0x%X site 0x%X (%s)" % (fs, r, ida_name.get_name(fs) or ""))
        r = nxt(DM, r)

# Find the page-select helper sub_1441ECDB0 callers (cargo tab builder)
print("=== callers of sub_1441ECDB0 (cargo page select) ===")
PS = 0x1441ECDB0
seen = set()
r = ida_xref.get_first_cref_to(PS)
while r != idaapi.BADADDR:
    f = ida_funcs.get_func(r)
    if f and f.start_ea not in seen:
        seen.add(f.start_ea)
        print("page-select caller 0x%X (%s) size=%d" % (
            f.start_ea, ida_name.get_name(f.start_ea) or "", f.end_ea - f.start_ea))
    r = ida_xref.get_next_cref_to(PS, r)

idc.qexit(0)

#!/usr/bin/env python3
"""df19: map cargo tab index -> panel object, via the cell factory.

df18 located the damage-font panel as the sub-object at window+9760 (its class
vtable is off_14A230DE8, slot +16 = the builder sub_1441CBDD0 that loads dstr
100002264 "You have no Damage fonts."). df16 showed every tab's cells are made by
sub_1441E0820(window, tab, id), one call site per tab constant, so that factory
must switch tab -> panel offset. Decompiling it closes tab == damage font.
"""
import ida_bytes
import ida_funcs
import ida_hexrays
import ida_name
import idautils
import idc

for ea, tag in ((0x1441E0820, "cell_factory(win,tab,id)"),
                (0x1441D9070, "df_panel_ctor_head"),
                (0x1444EC5B0, "mgr_page_users_1"),
                (0x1444EC930, "mgr_page3_user")):
    print("\n---- %s 0x%X ----" % (tag, ea))
    try:
        print(ida_hexrays.decompile(ea))
    except Exception as exc:
        print("  <failed %s>" % exc)

print("\n=== instructions that address window+0x2620 (9760, the damage-font panel) ===")
RANGES = [(0x1441B0000, 0x144200000), (0x1444E0000, 0x144500000)]
for lo, hi in RANGES:
    for f in idautils.Functions(lo, hi):
        for head in idautils.FuncItems(f):
            txt = idc.generate_disasm_line(head, 0) or ""
            if "2620h" in txt and ("+" in txt):
                print("  0x%X in %-24s : %s" % (head, ida_name.get_name(f) or "?", txt.split("\n")[0]))

idc.qexit(0)

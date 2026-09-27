#!/usr/bin/env python3
"""df16: pin down which cargo page index is the damage-font tab.

df15 showed the opcode-name table is not a dispatch table, so the remaining
question is purely client-side: the cargo manager owns 10 page containers at
mgr+136+16*page (ctor `sub_1444E81C0`), NOTI1545 fills page `mode` (or pages
0,1,2,3,4,5,7,8,9 for mode 10) and the skin-cargo window's tab refresh
`sub_1441ECDB0(win, tab)` reads tab `tab` straight back out with
`sub_1444EBC10(mgr, vec, tab)`. So tab index == page index; we only need the
index the damage-font panel uses.

This dumps: the page accessor / page vector readers and every call site with
the immediate the caller passes, plus the callers of the damage-font panel
builder `sub_1441CBDD0` (dstr 100002264 "You have no Damage fonts.").
"""
import ida_bytes
import ida_funcs
import ida_name
import idautils
import idc

PAGE_LOOKUP = 0x1444EBD90      # sub_1444EBD90(mgr, page, id) -> entry
PAGE_VEC = 0x1444EBAB0         # sub_1444EBAB0(id) -> entry (single map?)
PAGE_IDS = 0x1444EBC10         # sub_1444EBC10(mgr, out, page) -> id vector
TAB_REFRESH = 0x1441ECDB0      # sub_1441ECDB0(win, tab)
TAB_CELL = 0x1441E0820         # sub_1441E0820(win, tab, id)
DF_PANEL = 0x1441CBDD0         # damage-font panel builder
OWNED_PRED = 0x1444EC2C0       # sub_1444EC2C0(entry) -> expired/hidden flag


def arg_before(ea, reg):
    """Walk back up to 24 instructions looking for the last write to `reg`."""
    seen = []
    cur = ea
    for _ in range(24):
        cur = idc.prev_head(cur)
        if cur == idc.BADADDR:
            break
        line = idc.generate_disasm_line(cur, 0) or ""
        first = line.split("\n")[0].strip()
        seen.append(first)
        if first.lower().startswith(("mov", "xor", "lea", "cmov")) and reg.lower() in first:
            dst = first.split(None, 1)[1].split(",")[0].strip().lower()
            if dst == reg.lower():
                return first, seen
    return "", seen


def call_sites(target, regs=("edx", "r9d", "r8d")):
    print("\n=== call sites of 0x%X (%s) ===" % (target, ida_name.get_name(target) or "?"))
    for x in idautils.XrefsTo(target, 0):
        f = ida_funcs.get_func(x.frm)
        fname = ida_name.get_name(f.start_ea) if f else "?"
        dis = (idc.generate_disasm_line(x.frm, 0) or "").split("\n")[0]
        print("  0x%X in %s : %s" % (x.frm, fname, dis))
        for reg in regs:
            found, _ = arg_before(x.frm, reg)
            if found:
                print("        %-4s <- %s" % (reg, found))


def decompile(ea, tag):
    print("\n=== decompile %s 0x%X ===" % (tag, ea))
    c = idc.decompile(ea)
    print(c if c else "  <failed>")


for t in (PAGE_LOOKUP, PAGE_VEC, PAGE_IDS, TAB_REFRESH, TAB_CELL, DF_PANEL):
    call_sites(t)

decompile(PAGE_IDS, "page_ids")
decompile(PAGE_VEC, "page_vec")

print("\n=== damage-font panel: dstr users near sub_1441CBDD0 ===")
for x in idautils.XrefsTo(DF_PANEL, 0):
    f = ida_funcs.get_func(x.frm)
    print("  caller 0x%X (%s)" % (x.frm, ida_name.get_name(f.start_ea) if f else "?"))

print("\n=== functions referencing the cargo manager singleton 0x14E638F28 ===")
hits = {}
for x in idautils.XrefsTo(0x14E638F28, 0):
    f = ida_funcs.get_func(x.frm)
    if f:
        hits.setdefault(f.start_ea, 0)
        hits[f.start_ea] += 1
for ea in sorted(hits, key=lambda k: -hits[k])[:40]:
    print("  0x%X %-34s refs=%d size=%d" % (ea, ida_name.get_name(ea) or "?", hits[ea],
                                            ida_funcs.get_func(ea).end_ea - ea))

idc.qexit(0)

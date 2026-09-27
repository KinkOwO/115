#!/usr/bin/env python3
"""Decisive pass for the damage-font skin-cargo DISPLAY question.

Goals:
 1. Find callers of the CMD1565 SELECT_SKIN handler sub_1444E8D00 -> the body
    reader that decodes (category a2, index a3). Decompile caller(s).
 2. Decompile sub_1444EE820 (general skin select, a2!=0 branch).
 3. Find callers/registrar of NOTI1545 handler sub_1444ED900 and NOTI1547 handler
    sub_1444ED400 and dump the surrounding disasm so the registered id literal
    (or computed id) is visible.
 4. Decompile the skin-cargo manager ctor sub_1444E7F30 / sub_1444E81C0 to map
    per-page tree offsets (136+16*page) and any category labels.
 5. For each cargo page tree offset 136+16*p (p=0..9), find READ xrefs (functions
    that consume that page for rendering) to tie a page index to damage fonts.
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


def callers_of(va):
    out = set()
    for getter, nxt in ((ida_xref.get_first_cref_to, ida_xref.get_next_cref_to),
                        (ida_xref.get_first_dref_to, ida_xref.get_next_dref_to)):
        r = getter(va)
        while r != idaapi.BADADDR:
            f = ida_funcs.get_func(r)
            if f is not None:
                out.add((f.start_ea, r))
            r = nxt(va, r)
    return sorted(out)


def dump_func(fea, tag):
    name = ida_name.get_name(fea) or "sub_%X" % fea
    try:
        cf = ida_hexrays.decompile(fea)
    except Exception as exc:
        print("FAIL decompile %s %s %r" % (tag, name, exc))
        return
    if cf is None:
        print("NONE decompile %s %s" % (tag, name))
        return
    text = str(cf)
    p = os.path.join(OUT_DIR, "df6_%s_%s.c" % (tag, ("%X" % fea).lower()))
    open(p, "w", encoding="utf-8").write(text)
    print("OK %s %s lines=%d -> %s" % (tag, name, text.count("\n"), p))


def disasm_around(ea, back=0x40, fwd=0x10, tag=""):
    lo = ea - back
    lines = []
    cur = lo
    while cur < ea + fwd:
        lines.append("%X: %s" % (cur, idc.generate_disasm_line(cur, 0)))
        cur = idc.next_head(cur, ea + fwd + 0x20)
        if cur == idaapi.BADADDR:
            break
    print("--- disasm %s around %X ---" % (tag, ea))
    for ln in lines:
        print(ln)


# 1. CMD1565 handler callers (body reader / dispatch)
CMD1565 = 0x1444E8D00
print("=== callers of CMD1565 handler sub_1444E8D00 ===")
for fea, site in callers_of(CMD1565):
    print("caller func 0x%X site 0x%X (%s)" % (fea, site, ida_name.get_name(fea) or ""))
    dump_func(fea, "cmd1565_caller")

# 2. general skin select
print("=== sub_1444EE820 (general skin select) ===")
dump_func(0x1444EE820, "select_general")

# 3. NOTI1545 / NOTI1547 registrar sites
for tag, h in (("noti1545", 0x1444ED900), ("noti1547", 0x1444ED400),
               ("noti1546", 0x1444ED4F0)):
    print("=== callers/refs of %s handler %X ===" % (tag, h))
    for fea, site in callers_of(h):
        print("%s ref func 0x%X site 0x%X (%s)" % (tag, fea, site, ida_name.get_name(fea) or ""))
        disasm_around(site, 0x50, 0x10, tag)

# 4. cargo mgr ctors
print("=== cargo mgr ctors ===")
dump_func(0x1444E7F30, "mgr_ctor_f30")
dump_func(0x1444E81C0, "mgr_ctor_81c0")

idc.qexit(0)

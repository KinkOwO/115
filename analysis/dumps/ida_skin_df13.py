#!/usr/bin/env python3
"""df13: skin-cargo page semantics + the C2S trigger that opens the cargo.

Blocking questions for making a registered damage font visible in the client's
skin repository:

Q1/Q2  NOTI1545 body reader sub_1444EFF40 and NOTI1546 body reader
       sub_1444EECA0: exactly which manager member each page writes, and what
       the u32/u64 values mean (id vs flag).
Q3     NOTI1547 handler sub_1444ED400 and every consumer of the vector it
       fills (claimed offset 1096).
Q4     Which C2S opcodes the skin-cargo code sends: every call of the packet
       begin routine sub_146D746E0 whose `edx` immediate is set inside the
       skin-cargo manager range or the cargo tab controllers.

Output: analysis/dumps/skin-noti/df13_*.c plus a printed report.
"""
import os
import re
import ida_hexrays
import ida_funcs
import ida_lines
import ida_name
import ida_xref
import idaapi
import idautils
import idc

OUT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "skin-noti")
os.makedirs(OUT, exist_ok=True)

BEGIN_CMD = 0x146D746E0      # packet begin, edx = opcode id
READ_U8 = 0x146EA09F0
READ_U16 = 0x146EA1920
READ_U32 = 0x146EA0BA0
READ_U64 = 0x146EA0BE0

# code regions owned by the skin cargo manager and its tab controllers
REGIONS = [
    ("cargo_mgr", 0x1444E8000, 0x1444F3000),
    ("cargo_ui", 0x1441CB000, 0x1441EE000),
]


def dump(fea, tag):
    if not fea:
        return None
    nm = ida_name.get_name(fea) or "sub_%X" % fea
    try:
        cf = ida_hexrays.decompile(fea)
    except Exception as exc:
        print("FAIL %s %s %r" % (tag, nm, exc))
        return None
    if cf is None:
        print("NONE %s %s" % (tag, nm))
        return None
    text = str(cf)
    p = os.path.join(OUT, "df13_%s_%s.c" % (tag, ("%X" % fea).lower()))
    open(p, "w", encoding="utf-8").write(text)
    print("DUMP %s %s lines=%d -> %s" % (tag, nm, text.count("\n"), p))
    return text


print("=== Q1/Q2/Q3: noti body readers and handlers ===")
for tag, fea in (("noti1545_body", 0x1444EFF40),
                 ("noti1546_body", 0x1444EECA0),
                 ("noti1547_handler", 0x1444ED400),
                 ("noti1545_handler", 0x1444ED900),
                 ("noti1546_handler", 0x1444ED4F0),
                 ("cargo_ctor", 0x1444E81C0)):
    dump(fea, tag)

print("")
print("=== Q4a: packet opcodes sent from the skin-cargo code regions ===")


def edx_imm_before(call_ea, func):
    ea = call_ea
    for _ in range(40):
        ea = idc.prev_head(ea, func.start_ea)
        if ea == idc.BADADDR or ea < func.start_ea:
            return None
        txt = ida_lines.tag_remove(idc.generate_disasm_line(ea, 0) or "")
        m = re.match(r"mov\s+edx,\s*([0-9A-Fa-f]+)h", txt)
        if m:
            return int(m.group(1), 16)
        if re.match(r"xor\s+edx,\s*edx", txt):
            return 0
    return None


found = {}
for xr in idautils.XrefsTo(BEGIN_CMD, 0):
    frm = xr.frm
    f = ida_funcs.get_func(frm)
    if not f:
        continue
    region = None
    for nm, lo, hi in REGIONS:
        if lo <= f.start_ea < hi:
            region = nm
    if region is None:
        continue
    found.setdefault((region, f.start_ea), [])
    found[(region, f.start_ea)].append((frm, edx_imm_before(frm, f)))

for (region, fs), hits in sorted(found.items()):
    nm = ida_name.get_name(fs) or "sub_%X" % fs
    print("  [%s] 0x%X %s sends: %s" % (region, fs, nm,
          ", ".join("0x%X=>%s" % (ea, "None" if op is None else "%d(0x%X)" % (op, op)) for ea, op in hits)))

print("")
print("=== Q4b: functions inside the cargo regions (orientation) ===")
for nm, lo, hi in REGIONS:
    ea = lo
    n = 0
    while ea < hi and n < 400:
        f = ida_funcs.get_func(ea)
        if f is None:
            ea = ea + 16
            continue
        size = f.end_ea - f.start_ea
        if size > 40:
            callers = len(list(idautils.XrefsTo(f.start_ea, 0)))
            print("  %-12s 0x%X %-28s size=%-6d xrefs=%d" % (nm, f.start_ea,
                  ida_name.get_name(f.start_ea) or "?", size, callers))
        ea = f.end_ea
        n += 1

print("")
print("=== Q3b: consumers of the manager vector offsets (1096 / 136+16p) ===")
for tag, fea in (("cargo_render_tail", 0x1444F1EE0),
                 ("cargo_tree_insert", 0x1444E8030),
                 ("mapper_skin_id", 0x1473A1120),
                 ("cmd1565_branch", 0x1444EC790)):
    dump(fea, tag)

idc.qexit(0)

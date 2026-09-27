#!/usr/bin/env python3
"""df14: which skin-cargo page is the damage-font page?

df13 established:
  * manager ctor sub_1444E81C0 builds `mgr+136` as a 10-element page vector and
    registers NOTI1545/1546/1547 + CMD1565.
  * NOTI1545 body sub_1444EFF40 fills exactly one page tree per page index.
  * No skin-cargo UI sends a C2S request (only SELECT_SKIN 1565 / USE_SPRAY 2140
    / emote commands), so the cargo is server-pushed.

What is still unknown is the page index the damage-font tab reads. Angle:
  A) `sub_1444EBD90(mgr, page, id)` is the page-keyed lookup used by the NOTI1546
     reader. Enumerate every caller together with the constant page argument and
     the caller's name/region -> the UI functions will name their own page.
  B) sub_1441CBDD0 is the damage-font tab (it sets dstr 100002264 "You have no
     Damage fonts."). Decompile its callers and any function that both calls a
     page accessor and references damage-font dstr ids.
  C) dump the accessor bodies so the page argument position is certain.
"""
import os
import re
import ida_hexrays
import ida_funcs
import ida_lines
import ida_name
import idautils
import idc

OUT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "skin-noti")
os.makedirs(OUT, exist_ok=True)

PAGE_LOOKUP = 0x1444EBD90
DF_TAB = 0x1441CBDD0
DF_DSTR = {19662, 19666, 19667, 100002106, 100002108, 100002109, 100002110,
           100002111, 100002264, 101035806, 101035809}


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
    p = os.path.join(OUT, "df14_%s_%s.c" % (tag, ("%X" % fea).lower()))
    open(p, "w", encoding="utf-8").write(text)
    print("DUMP %s %s lines=%d -> %s" % (tag, nm, text.count("\n"), p))
    return text


print("=== C) accessor bodies (page argument position) ===")
for tag, fea in (("page_lookup", PAGE_LOOKUP),
                 ("page_flag", 0x1444EC2C0),
                 ("cargo_small", 0x1444EC320),
                 ("noti1547_writer", 0x1444E7F30)):
    dump(fea, tag)

print("")
print("=== A) every caller of the page lookup with its constant page argument ===")


def second_arg_imm(call_ea, func):
    """Heuristic: last immediate written to edx/rdx before the call."""
    ea = call_ea
    for _ in range(60):
        ea = idc.prev_head(ea, func.start_ea)
        if ea == idc.BADADDR or ea < func.start_ea:
            return None
        txt = ida_lines.tag_remove(idc.generate_disasm_line(ea, 0) or "")
        m = re.match(r"(?:mov|r8d-mov)\s+(?:e|r)?dx,\s*([0-9A-Fa-f]+)h", txt)
        if m:
            return int(m.group(1), 16)
        if re.match(r"xor\s+(?:e|r)dx,\s*(?:e|r)dx", txt):
            return 0
        if re.search(r"lea\s+(?:e|r)dx,", txt):
            return "lea"
    return None


by_func = {}
for xr in idautils.XrefsTo(PAGE_LOOKUP, 0):
    f = ida_funcs.get_func(xr.frm)
    if not f:
        continue
    by_func.setdefault((f.start_ea, f.end_ea), []).append(
        (xr.frm, second_arg_imm(xr.frm, f)))

for (fs, fe), hits in sorted(by_func.items()):
    nm = ida_name.get_name(fs) or "sub_%X" % fs
    pages = sorted({str(h) for _, h in hits}, key=str)
    print("  0x%X %-30s size=%-6d pages=%s" % (fs, nm, fe - fs, pages))

print("")
print("=== B) damage-font tab callers + dstr-referencing list builders ===")
for xr in idautils.XrefsTo(DF_TAB, 0):
    f = ida_funcs.get_func(xr.frm)
    if not f:
        print("  xref 0x%X (no function)" % xr.frm)
        continue
    print("  DF_TAB called from 0x%X (%s)" % (f.start_ea, ida_name.get_name(f.start_ea)))
    dump(f.start_ea, "dftab_caller")

DSTR_CONV = 0x14723C170
print("")
print("=== functions that pass a damage-font dstr id ===")
seen = set()
for xr in idautils.XrefsTo(DSTR_CONV, 0):
    f = ida_funcs.get_func(xr.frm)
    if not f or f.start_ea in seen:
        continue
    seen.add(f.start_ea)
    ea = f.start_ea
    ids = set()
    n = 0
    while ea < f.end_ea and n < 20000:
        n += 1
        if idc.print_insn_mnem(ea) == "mov" and idc.get_operand_type(ea, 1) == idc.o_imm:
            v = idc.get_operand_value(ea, 1)
            if v in DF_DSTR:
                ids.add(v)
        ea = idc.next_head(ea, f.end_ea)
    if not ids:
        continue
    print("  0x%X %s dstr=%s" % (f.start_ea, ida_name.get_name(f.start_ea) or "?", sorted(ids)))
    dump(f.start_ea, "dfuser")

idc.qexit(0)

#!/usr/bin/env python3
"""Pin the damage-font cargo PAGE by finding the function that shows
dstr 100002264 'You have no Damage fonts.' (the damage-font page empty
message). That function reads the damage-font page tree -> the page index.

dstr ids are materialized as immediates passed to sub_14723C170 (id->str),
as seen in sub_1444EC790. Scan the whole binary for `mov reg, <dstr_id>`
for the damage-font ids of interest and decompile the enclosing functions,
flagging any cargo page-tree offset (136+16*p) they touch.
"""
import os
import ida_hexrays
import ida_funcs
import ida_name
import ida_segment
import idaapi
import idc

OUT_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "skin-noti")
os.makedirs(OUT_DIR, exist_ok=True)

DSTR = {
    100002264: "You have no Damage fonts.",
    100002243: "df name default",
    100002249: "df name idx1",
    19666: "Damage Font",
    19662: "Changed the Damage font.",
}
PAGE_OFF = {136 + 16 * p: p for p in range(10)}
STR_CONV = 0x14723C170


def _find_binary(start, end, pat):
    for fn in (getattr(idaapi, "find_binary", None),
               getattr(__import__("ida_bytes"), "find_binary", None),
               getattr(__import__("ida_bytes"), "bin_search", None),
               getattr(idc, "find_binary", None)):
        if fn is None:
            continue
        try:
            r = fn(start, end, pat)
            if isinstance(r, tuple):
                r = r[0]
            if r is not None and r != idaapi.BADADDR:
                return r
        except Exception:
            continue
        try:
            r = fn(start, pat, 16, idaapi.SEARCH_DOWN)
            if isinstance(r, tuple):
                r = r[0]
            if r is not None and r != idaapi.BADADDR and r < end:
                return r
        except Exception:
            continue
    return None


def scan_imms():
    """Find `mov reg, <dstr_id>` by searching the imm32 LE bytes, then walking
    back to the instruction start. Distinctive high bytes (F5 05) keep the
    100002xxx ids nearly false-positive free."""
    hits = {}
    n = ida_segment.get_segm_qty()
    segs = []
    for i in range(n):
        seg = ida_segment.getnseg(i)
        if seg is None:
            continue
        segs.append((seg.start_ea, seg.end_ea, seg.perm))
    for v, label in DSTR.items():
        pat = " ".join("%02X" % b for b in (v & 0xFF, (v >> 8) & 0xFF, (v >> 16) & 0xFF, (v >> 24) & 0xFF))
        for lo, hi, perm in segs:
            if not (perm & ida_segment.SEGPERM_EXEC):
                continue
            cur = lo
            while cur < hi:
                ea = _find_binary(cur, hi, pat)
                if ea is None or ea == idaapi.BADADDR or ea >= hi:
                    break
                # walk back to the instruction head that owns these imm bytes
                head = ea
                for _ in range(4):
                    ph = idc.prev_head(head, head - 8)
                    if ph == idaapi.BADADDR:
                                break
                    if idc.get_operand_type(ph, 1) == idc.o_imm and idc.get_operand_value(ph, 1) == v:
                        head = ph
                        break
                    head = ph
                hits.setdefault(v, []).append(head)
                cur = ea + 4
    return hits


hits = scan_imms()
print("=== dstr-id immediate hits ===")
for v, eas in sorted(hits.items()):
    print("dstr %d (%s): %d hits" % (v, DSTR[v], len(eas)))
    for ea in eas[:8]:
        f = ida_funcs.get_func(ea)
        fs = f.start_ea if f else 0
        print("   @ %X in func 0x%X (%s)" % (ea, fs, ida_name.get_name(fs) or ""))

# Decompile the functions referencing the empty-damage-font message
print("=== decompile damage-font render functions ===")
done = set()
for v in (100002264, 19666, 19662):
    for ea in hits.get(v, []):
        f = ida_funcs.get_func(ea)
        if not f or f.start_ea in done:
            continue
        done.add(f.start_ea)
        nm = ida_name.get_name(f.start_ea) or "sub_%X" % f.start_ea
        try:
            cf = ida_hexrays.decompile(f.start_ea)
        except Exception as exc:
            print("FAIL %s %r" % (nm, exc)); continue
        if cf is None:
            continue
        text = str(cf)
        p = os.path.join(OUT_DIR, "df11_dfrender_%s.c" % ("%X" % f.start_ea).lower())
        open(p, "w", encoding="utf-8").write(text)
        # flag page-tree offsets touched
        pages = sorted({PAGE_OFF[d] for d in PAGE_OFF if ("+ %d)" % d) in text or ("+%d)" % d) in text or (" 0x%X)" % d) in text})
        print("OK %s lines=%d pages_maybe=%s -> %s" % (nm, text.count("\n"), pages, p))

idc.qexit(0)

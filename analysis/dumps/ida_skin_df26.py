#!/usr/bin/env python3
"""df26: who actually reads the applied damage-font, and what re-syncs it.

df25 closed four things:
- sub_142581F20(a,id) is a bare field write: *(a+232)=id. It is called on the
  local player (sub_143C61150) by NOTI1546 cat 6 and by the CMD1565 echo page 6,
  and on the 336-byte global holder qword_14E63AE60 by the skin-warehouse window
  handlers sub_1441D21E0 / sub_1441D23A0.
- Those two window handlers derive the value the same way every time:
  sub_1444EBC10(mgr, out, 6) -> sub_142581F20(qword_14E63AE60, *out.begin()),
  i.e. the panel's applied damage font is the FIRST id of the category-6
  selection vector stored at mgr+296[6].
- sub_1447EF2F0(a,id) (NOTI1546 cat 2 / echo page 2) is much heavier:
  *(a+112)=id, then it fires UI event 178 through qword_14E683C68 and pushes the
  value with sub_146664340.
- Only one function in the whole skin-manager cluster reads the applied set at
  mgr+312: sub_1444EC570(mgr,id) is a membership predicate.

Live 2026-09-27: the server replies NOTI1546 cat 6 (player+232 and mgr+296[6]
both land, the panel confirms the first apply), yet pressing 应用 on another font
does not switch it, and inside a dungeon the damage numbers keep the default
font. So the missing step is downstream of those writes.

This round answers three questions and nothing else:
1. Which functions read the holder global qword_14E63AE60 (and therefore the
   +232 field that the battle renderer would use)?
2. Who calls sub_1444EBC10 (the category-6 selection getter) - is the re-sync
   only on window open/close, or also on map/dungeon entry?
3. Who calls sub_1441D21E0 and sub_1441D23A0, i.e. what client events run the
   panel sync path?
"""
import os
import re
import ida_funcs
import ida_hexrays
import ida_name
import idautils
import idc

OUT = r"D:\115us\analysis\dumps\skin-noti"

HOLDER = 0x14E63AE60
GETTERS = (0x1444EBC10, 0x1441D21E0, 0x1441D23A0)
SIZE_CAP = 9000


def body_of(ea):
    try:
        return str(ida_hexrays.decompile(ea))
    except Exception:
        return ""


def dump(tag, text):
    name = "df26_%s.c" % re.sub(r"\W+", "_", tag)
    path = os.path.join(OUT, name)
    with open(path, "w", encoding="utf-8") as h:
        h.write("// %s\n\n%s\n" % (tag, text))
    return name


def callers_of(ea):
    out = []
    for xr in idautils.XrefsTo(ea, 0):
        if idc.print_insn_mnem(xr.frm) != "call":
            continue
        f = ida_funcs.get_func(xr.frm)
        if f and f.start_ea not in out:
            out.append(f.start_ea)
    return out


print("=== 1. references to the font holder global qword_14E63AE60 ===", flush=True)
seen = set()
for xr in idautils.XrefsTo(HOLDER, 0):
    f = ida_funcs.get_func(xr.frm)
    if not f or f.start_ea in seen:
        continue
    seen.add(f.start_ea)
    size = f.end_ea - f.start_ea
    n = ida_name.get_name(f.start_ea)
    print("ref at 0x%x in %s (0x%x, %d bytes)" % (xr.frm, n, f.start_ea, size), flush=True)
    if size > SIZE_CAP:
        print("  skip body, too large", flush=True)
        continue
    b = body_of(f.start_ea)
    print("  -> %s (%d chars)" % (dump("holder_%s_0x%x" % (n, f.start_ea), b), len(b)), flush=True)

print("=== 2. callers of the category-6 selection getter sub_1444EBC10 ===", flush=True)
for c in callers_of(0x1444EBC10):
    n = ida_name.get_name(c)
    size = ida_funcs.get_func(c).end_ea - c
    print("caller %s (0x%x, %d bytes)" % (n, c, size), flush=True)
    if size > SIZE_CAP:
        print("  skip body, too large", flush=True)
        continue
    b = body_of(c)
    print("  -> %s (%d chars)" % (dump("ebc10caller_%s_0x%x" % (n, c), b), len(b)), flush=True)

print("=== 3. callers of the panel sync handlers ===", flush=True)
for target in GETTERS[1:]:
    print("--- 0x%x" % target, flush=True)
    for c in callers_of(target):
        n = ida_name.get_name(c)
        size = ida_funcs.get_func(c).end_ea - c
        print("caller %s (0x%x, %d bytes)" % (n, c, size), flush=True)
        if size > SIZE_CAP:
            print("  skip body, too large", flush=True)
            continue
        b = body_of(c)
        print("  -> %s (%d chars)" % (dump("sync_%s_0x%x" % (n, c), b), len(b)), flush=True)

print("=== 4. who calls the membership predicate sub_1444EC570 ===", flush=True)
for c in callers_of(0x1444EC570):
    n = ida_name.get_name(c)
    print("caller %s (0x%x)" % (n, c), flush=True)
    b = body_of(c)
    print("  -> %s (%d chars)" % (dump("ec570caller_%s_0x%x" % (n, c), b), len(b)), flush=True)

print("=== 5. callers of the font resource builder sub_1444EAD50 ===", flush=True)
for c in callers_of(0x1444EAD50):
    n = ida_name.get_name(c)
    size = ida_funcs.get_func(c).end_ea - c
    print("caller %s (0x%x, %d bytes)" % (n, c, size), flush=True)
    if size > SIZE_CAP:
        print("  skip body, too large", flush=True)
        continue
    b = body_of(c)
    print("  -> %s (%d chars)" % (dump("ead50caller_%s_0x%x" % (n, c), b), len(b)), flush=True)

print("=== df26 done ===", flush=True)

print("=== df26 done ===", flush=True)

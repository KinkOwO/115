#!/usr/bin/env python3
"""df25: why does 应用 not switch to another font, and why is the dungeon damage
number font unaffected?

Closed before this round:
- df24 found no sender for the CMD1565 echo; df7-registrar-table.json proves 1565
  is registered as BOTH a cmd handler (sub_1444E8D00) and a noti handler
  (sub_14399DD70), i.e. the same opcode is request and echo.
- sub_1444EE820 (echo body, 88 bytes: u32 page, u32 result, u32 ids[20]) case 6
  calls only sub_142581F20(player,id) and then sub_1444E8ED0(mgr,id) which inserts
  into the set at mgr+312, and the tail always calls sub_1444F1EE0(mgr,page).
- NOTI1546 sub_1444EECA0 case 6 calls sub_142581F20(player,id) AND
  sub_1447EF380(player,id,0), but never touches mgr+312 and never refreshes.
- Live 2026-09-27 run: server sent NOTI1546 cat 6 for ids 12 and 18, the client
  confirmed the first apply, but pressing 应用 on another font did not switch it
  and the dungeon damage numbers kept the default font.

This round asks three things and nothing else:
1. What do sub_142581F20 / sub_1447EF380 / sub_1447EF2F0 / sub_1447EF3B0 actually
   write (which object offsets)?
2. Who else calls sub_142581F20 and sub_1447EF380 - in particular is there an
   actor/dungeon-entry path that re-applies or resets the font?
3. Inside the skin-manager cluster, which functions read the applied set at
   mgr+312 (so we learn what the panel uses as the "already applied" state)?
"""
import os
import re
import ida_funcs
import ida_hexrays
import ida_name
import idautils
import idc

OUT = r"D:\115us\analysis\dumps\skin-noti"

SETTERS = (0x142581F20, 0x1447EF380, 0x1447EF2F0, 0x1447EF3B0)
CLUSTER_LO = 0x1444E8000
CLUSTER_HI = 0x1444F2200
# Already analysed at length; decompiling them again only burns minutes.
SKIP = {0x1444EE820, 0x1444EECA0, 0x1444ED4F0, 0x1444ED900, 0x1444EFF40,
        0x1444E8D00, 0x1444E81C0, 0x1444E8ED0, 0x1444F1EE0}


def body_of(ea):
    try:
        return str(ida_hexrays.decompile(ea))
    except Exception:
        return ""


def dump(tag, text):
    name = "df25_%s.c" % re.sub(r"\W+", "_", tag)
    path = os.path.join(OUT, name)
    with open(path, "w", encoding="utf-8") as h:
        h.write("// %s\n\n%s\n" % (tag, text))
    return name


print("=== 1. setter bodies ===", flush=True)
for ea in SETTERS:
    b = body_of(ea)
    n = dump("setter_0x%x" % ea, b)
    print("0x%x %s -> %s (%d chars)" % (ea, ida_name.get_name(ea), n, len(b)), flush=True)

print("=== 2. callers of the setters ===", flush=True)
seen = set()
for ea in SETTERS:
    for xr in idautils.XrefsTo(ea, 0):
        frm = xr.frm
        if idc.print_insn_mnem(frm) != "call":
            continue
        f = ida_funcs.get_func(frm)
        if not f or f.start_ea in seen:
            continue
        seen.add(f.start_ea)
        caller = ida_name.get_name(f.start_ea)
        print("caller of 0x%x: %s (0x%x)" % (ea, caller, f.start_ea), flush=True)
        b = body_of(f.start_ea)
        n = dump("caller_%s_0x%x" % (caller, f.start_ea), b)
        print("  -> %s (%d chars)" % (n, len(b)), flush=True)

print("=== 3. cluster readers of mgr+312 ===", flush=True)
hits = 0
scan = 0
for ea in idautils.Functions(CLUSTER_LO, CLUSTER_HI):
    if ea in SKIP:
        continue
    f = ida_funcs.get_func(ea)
    if not f:
        continue
    size = f.end_ea - f.start_ea
    if size > 6000:
        print("skip big 0x%x (%d bytes)" % (ea, size), flush=True)
        continue
    scan += 1
    b = body_of(ea)
    if re.search(r"\+\s*312\s*\)", b):
        hits += 1
        n = dump("mgr312_%s_0x%x" % (ida_name.get_name(ea), ea), b)
        print("0x%x %s reads +312 -> %s" % (ea, ida_name.get_name(ea), n), flush=True)
print("scanned %d cluster functions, %d read mgr+312" % (scan, hits), flush=True)
print("=== df25 done ===", flush=True)

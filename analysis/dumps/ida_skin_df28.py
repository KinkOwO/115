#!/usr/bin/env python3
"""df28: who feeds the five damage-font slots, and what frame fills them.

df27 closed three things:
- sub_1444EAD50(mgr,id,slot) is the DEFAULT damage-number builder: it writes the
  holder field (sub_142581F20 -> *(holder+232)) and creates the text object with
  sub_1447EB510(..., a8 = -1), which snapshots holder+232.
- sub_1444EAF10(mgr,id,slot) is the CUSTOM builder: it looks the id up in the
  static skin registry (sub_140283D60, requiring record+8 == 2, the damage-font
  family) and creates the object through sub_1447EBED0 with the id passed
  explicitly - holder+232 is never read on this path.
- Both are called from exactly one place, sub_1441BDCD0, which walks FIVE records
  of 400 bytes at a1+152 and branches per record: id == 99999999 -> default,
  otherwise -> custom. It has no call xref at all, only a vtable reference at
  0x14A230E30, so it is a method of the class whose vtable starts at off_14A230DE8
  (the skin-warehouse window class already pinned in df9/df22).

So the applied font only reaches a slot whose record id is the sentinel, and a
record carrying a concrete id ignores it. Live 2026-09-27: the plain numbers show
the applied font while the burst-style numbers keep their own look - one slot
fell back, another has its own id.

df27 also surfaced the frame that can fill those slots: NOTI1672's handler
sub_1444EDFD0 loops entries 1..count and for tag 10 calls
sub_1444EFA00(mgr, category, slot) for categories 1, 4, 6, 7 and 10, where
category 6 is exactly sub_1447EF380(holder, id, slot) - a per-slot damage font.
NOTI1546 (sub_1444EECA0) only ever writes slot 0.

This round answers three questions and nothing else:
1. Who else uses the holder slot maps (sub_1401C4620 on holder+120 / +136)?
   That is the only way a battle object can read a per-slot id.
2. What is in off_14A230DE8's vtable around slot 9, and which function is the
   ctor of that class?
3. Does the ctor (or any method of that class) write record+0 at +152+400*n, and
   does it read the holder slot map while doing so?
"""
import os
import re
import ida_funcs
import ida_hexrays
import ida_name
import idautils
import idc

OUT = r"D:\115us\analysis\dumps\skin-noti"

MAP_ACCESSOR = 0x1401C4620
VTABLE = 0x14A230DE8
RENDER = 0x1441BDCD0
SIZE_CAP = 20000

SLOTMAP = re.compile(r"sub_1401C4620\((.{1,24}?)\)")
POOL = re.compile(r"\+ 152\)|\+ 452\)|\+ 752\)|\+ 1052\)|\+ 1352\)|99999999")


def body_of(ea):
    try:
        return str(ida_hexrays.decompile(ea))
    except Exception:
        return ""


def dump(tag, text):
    name = "df28_%s.c" % re.sub(r"\W+", "_", tag)
    with open(os.path.join(OUT, name), "w", encoding="utf-8") as h:
        h.write("// %s\n\n%s\n" % (tag, text))
    return name


def owner(ea):
    return ida_funcs.get_func(ea)


print("=== 1. users of the holder slot maps (sub_1401C4620) ===", flush=True)
# sub_1401C4620 is a generic map accessor with a very large xref set, so this
# section only decompiles callers that live in the three regions that can matter
# here: the holder class (0x1447E....), the skin manager cluster (0x1444E....)
# and the warehouse window class (0x1441D.... / 0x1441E....).
REGIONS = ((0x1447E0000, 0x144800000), (0x1444E0000, 0x1444F3000),
           (0x1441B0000, 0x1441F0000))
seen = set()
total = 0
for xr in idautils.XrefsTo(MAP_ACCESSOR, 0):
    total += 1
    f = owner(xr.frm)
    if not f or f.start_ea in seen:
        continue
    if not any(lo <= f.start_ea < hi for lo, hi in REGIONS):
        continue
    seen.add(f.start_ea)
    n = ida_name.get_name(f.start_ea)
    size = f.end_ea - f.start_ea
    print("caller %s (0x%x, %d bytes)" % (n, f.start_ea, size), flush=True)
    if size > SIZE_CAP or len(seen) > 30:
        print("  skip body", flush=True)
        continue
    b = body_of(f.start_ea)
    hits = [l.strip() for l in b.splitlines() if "sub_1401C4620" in l]
    print("  -> %s" % dump("slotmap_%s_0x%x" % (n, f.start_ea), b), flush=True)
    for l in hits[:8]:
        print("     | %s" % l[:140], flush=True)
print("raw xrefs: %d, in-region users: %d" % (total, len(seen)), flush=True)

print("=== 2. the class vtable around the renderer method ===", flush=True)
for i in range(16):
    ea = VTABLE + 8 * i
    v = idc.get_qword(ea)
    print("slot %2d 0x%x -> 0x%x %s%s" % (i, ea, v, ida_name.get_name(v),
                                          " <-- renderer" if v == RENDER else ""), flush=True)
print("--- data references to the vtable (ctor sites) ---", flush=True)
ctors = []
for xr in idautils.XrefsTo(VTABLE, 0):
    f = owner(xr.frm)
    n = ida_name.get_name(xr.frm) if not f else ida_name.get_name(f.start_ea)
    print("ref from 0x%x in %s" % (xr.frm, n), flush=True)
    if f and f.start_ea not in ctors:
        ctors.append(f.start_ea)
for c in ctors:
    n = ida_name.get_name(c)
    f = owner(c)
    size = f.end_ea - c
    print("ctor candidate %s (0x%x, %d bytes)" % (n, c, size), flush=True)
    if size > SIZE_CAP:
        print("  skip body, too large", flush=True)
        continue
    b = body_of(c)
    print("  -> %s" % dump("ctor_%s_0x%x" % (n, c), b), flush=True)
    for l in [x.strip() for x in b.splitlines() if POOL.search(x)][:10]:
        print("     | %s" % l[:140], flush=True)

print("=== 3. the renderer method itself and its pool writes ===", flush=True)
b = body_of(RENDER)
print("  -> %s" % dump("renderer_sub_1441bdcd0", b), flush=True)
for l in [x.strip() for x in b.splitlines() if POOL.search(x)][:10]:
    print("     | %s" % l[:140], flush=True)
print("=== df28 done ===", flush=True)

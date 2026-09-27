#!/usr/bin/env python3
"""df30: find the real consumer of the per-slot damage font, without assuming regions.

df29 closed:
- The 400-byte pool element is an object: its initialiser sub_1441B98C0 writes a
  vptr (off_14A230C10) at element+0 and zeroes the rest, then calls
  sub_1441BFEC0. So the renderer's id read is element+40
  (sub_1441BDCD0 computes *(u32*)((DWORD*)v5 - 10) with v5 = element+80), and the
  pointer it matches against the incoming object is element+112.
- No method of the pool owner class off_14A230DE8 writes element+40; the only
  reader of it is the renderer sub_1441BDCD0 itself.
- The warehouse member keeps TWO text objects side by side: a1+3680 created with
  sub_1447EBED0(..., skin_id, 0) (explicit id) and a1+3688 created with
  sub_1447EB510(..., a8 = -1) (snapshot of holder+232). The panel handler
  sub_1441D23A0 creates the 3688 one first and only then writes holder+232.

df28's "the holder slot maps have no reader" claim is weaker than it looks: it
only decompiled callers inside three address ranges, while sub_1401C4620 has 843
raw xrefs. So the per-slot damage font may still be read by the battle HUD.

This round answers two questions and nothing else:
1. Which functions BOTH read the holder global qword_14E63AE60 and call the map
   accessor sub_1401C4620 - across the whole binary, no region filter. Those are
   the only candidates that can consult a per-slot font.
2. What are the methods of the pool element class off_14A230C10, and does one of
   them write element+40 (the id the renderer branches on)?
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
MAP_ACCESSOR = 0x1401C4620
ELEMENT_VTABLE = 0x14A230C10
ELEMENT_INIT_EXTRA = 0x1441BFEC0
SIZE_CAP = 20000

WANTED = re.compile(r"\+ 40\)|\+ 112\)|99999999|sub_142581F20|sub_143C61150"
                    r"|sub_1447EB510|sub_1447EBED0|sub_1401C4620")


def body_of(ea):
    try:
        return str(ida_hexrays.decompile(ea))
    except Exception:
        return ""


def dump(tag, text):
    name = "df30_%s.c" % re.sub(r"\W+", "_", tag)
    with open(os.path.join(OUT, name), "w", encoding="utf-8") as h:
        h.write("// %s\n\n%s\n" % (tag, text))
    return name


def funcs_with_ref(target):
    out = set()
    for xr in idautils.XrefsTo(target, 0):
        f = ida_funcs.get_func(xr.frm)
        if f:
            out.add(f.start_ea)
    return out


print("=== 1. functions that read the holder AND use a slot map ===", flush=True)
holders = funcs_with_ref(HOLDER)
mappers = funcs_with_ref(MAP_ACCESSOR)
both = sorted(holders & mappers)
print("holder readers: %d, map users: %d, intersection: %d" % (len(holders), len(mappers), len(both)), flush=True)
for ea in both:
    n = ida_name.get_name(ea)
    f = ida_funcs.get_func(ea)
    size = f.end_ea - ea
    print("%s (0x%x, %d bytes)" % (n, ea, size), flush=True)
    if size > SIZE_CAP:
        print("  skip body, too large", flush=True)
        continue
    b = body_of(ea)
    hits = [l.strip() for l in b.splitlines() if "sub_1401C4620" in l or "+ 136" in l or "+ 120" in l]
    print("  -> %s" % dump("both_%s_0x%x" % (n, ea), b), flush=True)
    for l in hits[:8]:
        print("     | %s" % l[:150], flush=True)

print("=== 2. the pool element class off_14A230C10 ===", flush=True)
for i in range(12):
    ea = idc.get_qword(ELEMENT_VTABLE + 8 * i)
    if not ea:
        continue
    f = ida_funcs.get_func(ea)
    if not f:
        print("slot %2d -> 0x%x %s (no function)" % (i, ea, ida_name.get_name(ea)), flush=True)
        continue
    n = ida_name.get_name(f.start_ea)
    size = f.end_ea - f.start_ea
    print("slot %2d -> %s (0x%x, %d bytes)" % (i, n, f.start_ea, size), flush=True)
    if size > SIZE_CAP:
        continue
    b = body_of(f.start_ea)
    print("  -> %s" % dump("elemmethod_%s_0x%x" % (n, f.start_ea), b), flush=True)
    for l in [x.strip() for x in b.splitlines() if WANTED.search(x)][:8]:
        print("     | %s" % l[:150], flush=True)
print("--- element extra initialiser ---", flush=True)
b = body_of(ELEMENT_INIT_EXTRA)
print("  -> %s (%d chars)" % (dump("eleminit_sub_1441bfec0", b), len(b)), flush=True)
for l in [x.strip() for x in b.splitlines() if WANTED.search(x)][:10]:
    print("     | %s" % l[:150], flush=True)

print("=== 3. who references the element vtable (its writers) ===", flush=True)
for xr in idautils.XrefsTo(ELEMENT_VTABLE, 0):
    f = ida_funcs.get_func(xr.frm)
    print("ref from 0x%x in %s" % (xr.frm, ida_name.get_name(f.start_ea) if f else "-"), flush=True)

print("=== df30 done ===", flush=True)

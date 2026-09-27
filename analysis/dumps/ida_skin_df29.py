#!/usr/bin/env python3
"""df29: who puts a concrete skin id into the five damage-number records.

df28 closed:
- The holder slot maps have no other user: in the holder class, the skin-manager
  cluster and the window class, sub_1401C4620 is called on holder+136 only by
  sub_1447EF380 (NOTI1546 cat 6 / NOTI1672 cat 6) and on holder+120 only by
  sub_1447EF3B0. So those maps are written and never read - they are not what the
  battle renderer consults.
- sub_1441BDCD0 is vtable slot 9 of off_14A230DE8, the class that lives at
  window+9760, and its ctor is sub_1441B8E90. That ctor builds THREE pools of
  five 400-byte records with sub_148860630(a1+5384 / a1+7536 / a1+9912, 400, 5,
  sub_1441B98C0, sub_1441BB860) - and 9912 - 9760 = 152, exactly the record base
  the renderer walks. So the renderer's pool is the third one, element size 400,
  five elements, with sub_1441B98C0 as the per-element initialiser.
- Both font builders (sub_1444EAD50 default, sub_1444EAF10 custom) are called from
  that renderer and nowhere else.

Live 2026-09-27: the plain damage numbers show the applied font, the burst-style
ones do not. Under the record model that means one record still holds the
99999999 sentinel (falls back to holder+232) while another holds a concrete id.

This round answers two questions and nothing else:
1. What does the element initialiser sub_1441B98C0 put in record+0, and what is
   the 400-byte record layout (which offset is the id the renderer reads)?
2. Which function in this class writes a NON-sentinel id into record+0 - i.e.
   every method of off_14A230DE8 plus the element helpers, grepped for the record
   base and for the two builders.
"""
import os
import re
import ida_funcs
import ida_hexrays
import ida_name
import idautils
import idc

OUT = r"D:\115us\analysis\dumps\skin-noti"

VTABLE = 0x14A230DE8
ELEMENT_INIT = 0x1441B98C0
ELEMENT_DTOR = 0x1441BB860
RENDER = 0x1441BDCD0
SIZE_CAP = 30000

WANTED = re.compile(r"\+ 152\)|99999999|sub_1444EAD50|sub_1444EAF10|sub_142581F20"
                    r"|sub_1447EF380|sub_143C61150|sub_1447EB510|sub_1447EBED0")


def body_of(ea):
    try:
        return str(ida_hexrays.decompile(ea))
    except Exception:
        return ""


def dump(tag, text):
    name = "df29_%s.c" % re.sub(r"\W+", "_", tag)
    with open(os.path.join(OUT, name), "w", encoding="utf-8") as h:
        h.write("// %s\n\n%s\n" % (tag, text))
    return name


def show(tag, ea):
    n = ida_name.get_name(ea)
    f = ida_funcs.get_func(ea)
    size = f.end_ea - ea if f else 0
    print("%s (0x%x, %d bytes)" % (n, ea, size), flush=True)
    if size > SIZE_CAP:
        print("  skip body, too large", flush=True)
        return
    b = body_of(ea)
    print("  -> %s (%d chars)" % (dump(tag + "_" + n + "_0x%x" % ea, b), len(b)), flush=True)
    for l in [x.strip() for x in b.splitlines() if WANTED.search(x)][:12]:
        print("     | %s" % l[:150], flush=True)


print("=== 1. the 400-byte record element helpers ===", flush=True)
for ea in (ELEMENT_INIT, ELEMENT_DTOR):
    show("element", ea)

print("=== 2. every method of the pool owner class off_14A230DE8 ===", flush=True)
done = set()
for i in range(24):
    ea = idc.get_qword(VTABLE + 8 * i)
    if not ea or ea in done:
        continue
    f = ida_funcs.get_func(ea)
    if not f:
        continue
    done.add(ea)
    print("--- vtable slot %d" % i, flush=True)
    show("method", f.start_ea)

print("=== 3. the renderer again, with its record reads annotated ===", flush=True)
show("render", RENDER)
print("=== df29 done ===", flush=True)

#!/usr/bin/env python3
"""df27: why one damage number layer follows the applied font and another does not.

df26 closed:
- sub_143C61150() is the lazy getter of the 336-byte global holder qword_14E63AE60,
  so NOTI1446 cat 6, the CMD1565 echo and the panel handlers all write the SAME
  *(u32*)(holder+232).
- sub_1447EB510(obj,...,a8) snapshots the font when the object is created:
  if (a8 <= -1) v91 = *(u32*)(sub_143C61150() + 232); *(obj+104) = v91.
- The renderer loop sub_1441BDCD0 is the caller of the default builder
  sub_1444EAD50.

Reading that loop's body afterwards shows the shape this round is about: it walks
FIVE records of 400 bytes each (v5 = a1+232, v5 += 50, v4 < 5), takes the skin id
at record+0 (i.e. *(a1+152+400*slot)) and branches on it:

    if (id == 99999999) sub_1444EAD50(mgr, id, slot)   # default look
    else                sub_1444EAF10(mgr, id, slot)   # custom look

So the applied font only reaches a record whose own id is the sentinel, and a
record that carries a concrete id ignores holder+232 entirely.

Live 2026-09-27 with the dungeon re-push in place: the plain damage numbers show
the applied font, while the big burst-style numbers keep their own look. That is
exactly "one record fell back, the other has its own id".

This round answers three questions and nothing else:
1. What do sub_1444EAD50 and sub_1444EAF10 each pass downstream as the font id,
   and does the custom builder read holder+232 at all?
2. Who calls sub_1441BDCD0 and sub_1444EAF10 - which object owns the 5-record
   pool, and is it a global (so its class functions are enumerable)?
3. Who writes record+0 (holder/pool +152 + 400*slot) - i.e. where does a
   non-sentinel id come from, and is it a NOTI1546 category, a PVF table, or a
   hardcoded default?
"""
import os
import re
import ida_funcs
import ida_hexrays
import ida_name
import idautils
import idc

OUT = r"D:\115us\analysis\dumps\skin-noti"

EAD50 = 0x1444EAD50
EAF10 = 0x1444EAF10
RENDER = 0x1441BDCD0
SLOT_SETTER = 0x1447EF380
SIZE_CAP = 20000

INTEREST = re.compile(r"\+ 152\)|\+ 452\)|\+ 752\)|99999999|sub_1447EF380"
                     r"|sub_142581F20|sub_143C61150|sub_1447EB510")


def body_of(ea):
    try:
        return str(ida_hexrays.decompile(ea))
    except Exception:
        return ""


def dump(tag, text):
    name = "df27_%s.c" % re.sub(r"\W+", "_", tag)
    with open(os.path.join(OUT, name), "w", encoding="utf-8") as h:
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


def emit(tag, ea, body):
    name = dump(tag, body)
    hits = [l.strip() for l in body.splitlines() if INTEREST.search(l)]
    print("  -> %s (%d chars)" % (name, len(body)), flush=True)
    for l in hits[:14]:
        print("     | %s" % l[:150], flush=True)


print("=== 1. the two builders: default vs custom font ===", flush=True)
for ea in (EAD50, EAF10):
    n = ida_name.get_name(ea)
    size = ida_funcs.get_func(ea).end_ea - ea
    print("%s (0x%x, %d bytes)" % (n, ea, size), flush=True)
    emit("builder_%s_0x%x" % (n, ea), ea, body_of(ea))

print("=== 2. callers of the renderer loop and of the custom builder ===", flush=True)
for target in (RENDER, EAF10):
    print("--- callers of 0x%x" % target, flush=True)
    for c in callers_of(target):
        n = ida_name.get_name(c)
        f = ida_funcs.get_func(c)
        size = f.end_ea - c
        print("caller %s (0x%x, %d bytes)" % (n, c, size), flush=True)
        if size > SIZE_CAP:
            print("  skip body, too large", flush=True)
            continue
        emit("caller_%s_0x%x" % (n, c), c, body_of(c))

print("=== 3. writers of the per-record id: callers of the slot map setter ===", flush=True)
for c in callers_of(SLOT_SETTER):
    n = ida_name.get_name(c)
    f = ida_funcs.get_func(c)
    size = f.end_ea - c
    print("caller %s (0x%x, %d bytes)" % (n, c, size), flush=True)
    if size > SIZE_CAP:
        print("  skip body, too large", flush=True)
        continue
    emit("slotsetter_%s_0x%x" % (n, c), c, body_of(c))

print("=== 4. xrefs to the renderer loop itself (non-call sites) ===", flush=True)
# The pool owner is reached through the caller's object pointer, so list every
# reference to the renderer (vtable slots included) instead of scanning .text:
# a vtable address tells us which class owns the pool.
for xr in idautils.XrefsTo(RENDER, 0):
    print("xref from 0x%x (%s) type=%d" % (xr.frm, idc.get_func_name(xr.frm), xr.type), flush=True)

print("=== df27 done ===", flush=True)

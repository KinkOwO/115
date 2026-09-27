#!/usr/bin/env python3
"""df31: does anything actually READ the per-slot damage-font maps?

df30 closed:
- The 5x400 pool element class off_14A230C10 has a setter in vtable slot 2
  (sub_1441E2620(element, id, category)) that writes element+40 after validating
  sub_1444EBAB0(id) -> record+8 == category (category 5 and 9 take other
  registries); the extra initialiser sub_1441BFEC0 sets element+40 = -1 and
  element+112 = 0. So element+40 is per-slot skin state, set through the element
  vtable, not by the pool owner class.
- The renderer sub_1441BDCD0 walks 5 records (element base = a1+192, stride 400),
  compares element+112 against the incoming object, then branches on element+40:
  99999999 -> default builder sub_1444EAD50, anything else -> custom builder
  sub_1444EAF10 (which uses the record's own id and ignores the applied font).
- df30's "intersection = 0" (holder-global readers vs map users) proves nothing
  about the slot maps: a holder method receives the holder pointer as an argument
  and never references qword_14E63AE60 itself.

This round answers one question: which functions call the map accessor
sub_1401C4620 with the holder's slot-map heads (a1+120 = category 2 state,
a1+136 = category 6 state), and who else ever touches those two setters
(sub_1447EF3B0 / sub_1447EF380). If a reader exists and is indexed by a slot
other than 0, NOTI1546 (slot 0 only) cannot change it and NOTI1672 (which passes
the entry index as the slot) can.

Second, small question: what does sub_1447EDD20(holder, id, ...) resolve an id
through - another slot map, or a plain table?
"""
import os
import re
import ida_funcs
import ida_hexrays
import ida_name
import idc
import idautils

OUT = r"D:\115us\analysis\dumps\skin-noti"

MAP_ACCESSOR = 0x1401C4620
SETTER_CAT2 = 0x1447EF3B0
SETTER_CAT6 = 0x1447EF380
RESOLVE_ID = 0x1447EDD20
HOLDER_GLOBAL = 0x14E63AE60
SIZE_CAP = 40000

CALL_RE = re.compile(r"sub_1401C4620\(\s*([^,]{1,60}),")
OFF_RE = re.compile(r"\+\s*(120|136)\b")
SLOT_RE = re.compile(r"\+\s*(\d+)\b")


def body_of(ea):
    try:
        return str(ida_hexrays.decompile(ea))
    except Exception:
        return ""


def var_assigns(body):
    """Map a local name to the expression it was last assigned, so an accessor
    called with a hoisted pointer (v3 = a1 + 136) still shows its head offset."""
    out = {}
    for m in re.finditer(r"^ {2}(v\d+|[a-z]\d*) = ([^;]{1,80});", body, re.M):
        out.setdefault(m.group(1), m.group(2).strip())
    return out


def dump(tag, text):
    name = "df31_%s.c" % re.sub(r"\W+", "_", tag)
    with open(os.path.join(OUT, name), "w", encoding="utf-8") as h:
        h.write("// %s\n\n%s\n" % (tag, text))
    return name


def caller_funcs(target):
    out = set()
    for xr in idautils.XrefsTo(target, 0):
        f = ida_funcs.get_func(xr.frm)
        if f:
            out.add(f.start_ea)
    return sorted(out)


print("=== 1. every caller of the map accessor, with the head offset it uses ===", flush=True)
users = caller_funcs(MAP_ACCESSOR)
print("map accessor caller functions: %d" % len(users), flush=True)
slot_hits = []
offsets = {}
for ea in users:
    n = ida_name.get_name(ea)
    f = ida_funcs.get_func(ea)
    size = f.end_ea - ea
    if size > SIZE_CAP:
        print("%s (0x%x, %d bytes) skip body" % (n, ea, size), flush=True)
        continue
    b = body_of(ea)
    args = CALL_RE.findall(b)
    if not args:
        continue
    va = var_assigns(b)
    heads = set()
    for a in args:
        a = a.strip()
        expr = a
        if re.fullmatch(r"(v\d+|[a-z]\d*)", a) and a in va:
            expr = va[a]
        nums = SLOT_RE.findall(expr)
        if nums:
            heads.update(nums)
        else:
            heads.add("dyn:%s" % expr[:36])
    for h in heads:
        offsets[h] = offsets.get(h, 0) + 1
    if "120" in heads or "136" in heads:
        slot_hits.append((ea, n, sorted(heads)))
        print("%s (0x%x) HEADS=%s  <== holder slot map" % (n, ea, sorted(heads)), flush=True)
print("--- head-offset histogram (arg expression -> function count) ---", flush=True)
for k, v in sorted(offsets.items(), key=lambda kv: -kv[1])[:40]:
    print("  %-42s %d" % (k, v), flush=True)
print("--- slot-map (120/136) consumers: %d ---" % len(slot_hits), flush=True)
for ea, n, heads in slot_hits:
    b = body_of(ea)
    print("  %s (0x%x) %s -> %s" % (n, ea, heads,
                                    dump("slotreader_%s_0x%x" % (n, ea), b)), flush=True)
    for l in [x.strip() for x in b.splitlines() if "sub_1401C4620" in x][:6]:
        print("     | %s" % l[:160], flush=True)

print("=== 2. who else calls the two slot setters ===", flush=True)
for tgt, label in ((SETTER_CAT2, "cat2 sub_1447EF3B0"), (SETTER_CAT6, "cat6 sub_1447EF380")):
    cs = caller_funcs(tgt)
    print("%s: %d caller functions" % (label, len(cs)), flush=True)
    for ea in cs:
        print("  %s (0x%x)" % (ida_name.get_name(ea), ea), flush=True)

print("=== 3. what sub_1447EDD20 resolves an id through ===", flush=True)
b = body_of(RESOLVE_ID)
print("  -> %s (%d chars)" % (dump("resolveid_sub_1447edd20", b), len(b)), flush=True)
for l in [x.strip() for x in b.splitlines()
          if re.search(r"sub_1401C4620|\+ (112|120|136|232)\)|qword_14E6", x)][:12]:
    print("     | %s" % l[:160], flush=True)

print("=== df31 done ===", flush=True)

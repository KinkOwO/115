#!/usr/bin/env python3
"""df23: how does the client learn a skin's page, and what does page 2 hold?

df22 closed the render path: the damage-font panel (class vtable off_14A230DE8, object
at window+9760) refills its grid in sub_1441E3510 by taking sub_1444EBDF0(mgr, 2) --
a snapshot of owned page 2 -- and keeping only the ids whose record in the global map
qword_14E683BF8 has +8 == 2. So NOTI1545 page 2 is the damage-font page, and the
registry is a *pre-existing* skin ID -> {page, sub type} table, because a missing id
would be inserted with page 0 by sub_1444EBAB0 and then filtered out.

If that table were built by a packet we do not send, a page-2 push would render an
empty grid. df23 therefore reads the manager's registry helpers and their callers to
find where +8/+12 are written, and documents the two page accessors used by the UI.
"""
import os
import re
import ida_bytes
import ida_funcs
import ida_hexrays
import ida_name
import idautils
import idc

OUT = r"D:\115us\analysis\dumps\skin-noti"
REGISTRY = 0x14E683BF8
INSERT = 0x140283D60


def body_of(ea):
    try:
        return str(ida_hexrays.decompile(ea))
    except Exception:
        return ""


def dump(tag, text):
    name = "df23_%s.c" % re.sub(r"\W+", "_", tag)
    with open(os.path.join(OUT, name), "w", encoding="utf-8") as handle:
        handle.write("// %s\n\n%s\n" % (tag, text))
    return name


print("=== 1. manager helpers that touch the registry, with their stores ===")
for ea in (0x1444EAD50, 0x1444EAF10, 0x1444EB0A0, 0x1444EB280, 0x1444EB5F0, 0x1444EB940,
           0x1444EBAB0, 0x1444EBDF0, 0x1444EC320, 0x1444EC530, 0x1444ECBD0, 0x1444EE820,
           0x1444F1920, 0x1444EC370, 0x1444EC410, 0x1444EC570, 0x1444EC6E0, 0x1444EC710,
           0x1444EAA90, 0x1444EAB00, 0x1444EBF80, 0x1444EBFC0):
    nm = ida_name.get_name(ea) or "?"
    body = body_of(ea)
    sig = body.split("\n")[0] if body else "?"
    stores = re.findall(r"\+\s*(8|12|16|20)\)\s*=\s*([^;]{1,24});", body)
    uses_registry = "E683BF8" in body or "sub_140283D60" in body
    n_callers = sum(1 for xr in idautils.XrefsTo(ea, 0) if idc.print_insn_mnem(xr.frm) == "call")
    print("  0x%X %-16s reg=%-5s callers=%-4d stores=%s" % (ea, nm, uses_registry, n_callers, stores[:6]))
    print("      %s" % sig[:110])
    if uses_registry or stores:
        dump(nm, body)

print("\n=== 2. every function in the image storing into a registry record (+8 / +12) ===")
hits = set()
for xr in idautils.XrefsTo(INSERT, 0):
    fn = ida_funcs.get_func(xr.frm)
    if fn:
        hits.add(fn.start_ea)
print("  %d functions call the map insert at all" % len(hits))
report = []
for ea in sorted(hits):
    body = body_of(ea)
    if not re.search(r"\+\s*12\)\s*=", body):
        continue
    if not re.search(r"\+\s*8\)\s*=\s*[0-9]", body):
        continue
    report.append(ea)
print("  functions storing small constants at +8 and something at +12: %s" % [hex(x) for x in report[:40]])
for ea in report[:12]:
    print("      0x%X %s -> %s" % (ea, ida_name.get_name(ea), dump("ins_" + (ida_name.get_name(ea) or "?"), body_of(ea))))

print("\n=== 3. callers of sub_1444EBDF0(mgr,page) with the page constant ===")
for xr in idautils.XrefsTo(0x1444EBDF0, 0):
    if idc.print_insn_mnem(xr.frm) != "call":
        continue
    fn = ida_funcs.get_func(xr.frm)
    consts, head = [], xr.frm
    for _ in range(6):
        head = idc.prev_head(head)
        if head == 0xFFFFFFFFFFFFFFFF:
            break
        line = (idc.generate_disasm_line(head, 0) or "").split("\n")[0]
        if re.search(r"mov\s+e[a-d]x,", line):
            consts.append(line.strip())
    print("  0x%X in %-16s %s" % (xr.frm, ida_name.get_name(fn.start_ea) if fn else "?", consts[:2]))

idc.qexit(0)

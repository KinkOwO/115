#!/usr/bin/env python3
"""df22: who assigns a skin's owned page, and which container the grid iterates.

df21 closed two things that reframe the problem:
  sub_1444EC1B0(mgr, itemTemplate) reads the item record, requires the action word
    at +2048 to be 169 ([add skin storage]), takes the skin id from the parameter
    vector at +2056, then resolves the page through the global map
    qword_14E683BF8: record+8 = owned page index, record+12 = sub type.
  sub_1441ECDB0(win, tab) builds ONE cell per category (the equipped slot), so the
    scrollable owned list must come from a page-map iterator instead.

So the missing server-side fact is not "which packet renders a cell" but "which page
does the client consider a damage font to live in, and who says so". df22 traces the
writers of that page/subtype record and the page-array consumers.
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
REGISTRY_INSERT = 0x140283D60
ENSURE_ID = 0x1444EBAB0
PAGE_ARRAY_USERS = (0x1444EBDF0, 0x1444EC0D0, 0x1444EC1B0, 0x1444EC320, 0x1444EC790,
                    0x1444ECFF0, 0x1444ED1B0, 0x1444ED400, 0x1444ED4F0, 0x1444EDFD0,
                    0x1444EE1C0, 0x1444EE4F0, 0x1444EFA00, 0x1444F0270, 0x1444F0FE0,
                    0x1444F1090, 0x1444F1310, 0x1444F1880, 0x1444F1EE0, 0x1444F1FC0,
                    0x1444F2030)


def body_of(ea):
    try:
        return str(ida_hexrays.decompile(ea))
    except Exception:
        return ""


def dump(tag, body):
    path = os.path.join(OUT, "df22_%s.c" % tag)
    with open(path, "w", encoding="utf-8") as handle:
        handle.write("// %s\n\n%s\n" % (tag, body))
    return os.path.basename(path)


print("=== 1. page-array users: signature and page constants at call sites ===")
for ea in PAGE_ARRAY_USERS:
    nm = ida_name.get_name(ea)
    if not nm:
        continue
    body = body_of(ea)
    sig = body.split("\n")[0] if body else "?"
    args = re.search(r"\(([^)]*)\)", sig)
    sites = [xr.frm for xr in idautils.XrefsTo(ea, 0) if idc.print_insn_mnem(xr.frm) == "call"]
    print("  0x%X %-16s %s callers=%d" % (ea, nm, (args.group(1) if args else "?")[:70], len(sites)))
    for site in sites[:6]:
        ctx = []
        head = site
        for _ in range(7):
            head = idc.prev_head(head)
            if head == 0xFFFFFFFFFFFFFFFF:
                break
            line = (idc.generate_disasm_line(head, 0) or "").split("\n")[0]
            if re.search(r"\b(e[a-d]x|r[0-9]+d)\s*,", line) or "call" in line:
                ctx.append(line.strip())
        print("      0x%X <- %s" % (site, " ; ".join(ctx[:5])))

print("\n=== 2. writers of the skin-id registry record (stores to +8 page / +12 subtype) ===")
candidates = set()
for xr in idautils.XrefsTo(ENSURE_ID, 0):
    fn = ida_funcs.get_func(xr.frm)
    if fn:
        candidates.add(fn.start_ea)
for xr in idautils.XrefsTo(REGISTRY_INSERT, 0):
    fn = ida_funcs.get_func(xr.frm)
    if fn and 0x1444E8000 <= fn.start_ea < 0x1444F2600:
        candidates.add(fn.start_ea)
for ea in sorted(candidates):
    body = body_of(ea)
    hits = re.findall(r"\+ (8|12|16|20)\)\s*=\s*([0-9A-Fa-f]{1,8})\w*\s*;", body)
    page_hits = [(o, v) for o, v in hits if v in ("1", "2", "3", "4", "5", "6", "7", "8", "9")]
    if not page_hits:
        continue
    name = ida_name.get_name(ea)
    print("  %-16s 0x%X  stores=%s -> %s" % (name, ea, page_hits[:8], dump("reg_" + name, body)))

print("\n=== 3. damage-font panel class vtable off_14A230DE8 ===")
for slot in range(14):
    ptr = ida_bytes.get_qword(0x14A230DE8 + 8 * slot)
    if not ptr or ptr < 0x140000000 or ptr > 0x149000000:
        continue
    body = body_of(ptr)
    used = [n for n in ("sub_1444EBD90", "sub_1444EBC10", "sub_1444EBDF0", "sub_1444ECFF0",
                        "sub_1444ED1B0", "sub_1444EE4F0", "sub_1444F0270", "sub_1444F1090")
            if n in body]
    print("  slot %2d 0x%X %-16s uses=%s" % (slot, ptr, ida_name.get_name(ptr) or "?", used or "-"))
    if used:
        print("      -> %s" % dump("dfvtable_%d_%X" % (slot, ptr), body))

print("\n=== 4. functions iterating a page map and pushing ids (candidate grid fillers) ===")
scanned = 0
for f in idautils.Functions(0x1441B0000, 0x144200000):
    scanned += 1
    if scanned > 500:
        print("  (scan capped at 500 functions)")
        break
    body = body_of(f)
    if "sub_1444EBD90" not in body and "sub_1444EBDF0" not in body:
        continue
    names = sorted(set(re.findall(r"sub_1444E[A-F0-9]{3,4}", body)))
    print("  0x%X %-16s %s" % (f, ida_name.get_name(f), names))
    if len(names) <= 6:
        dump("ui_%X" % f, body)

idc.qexit(0)

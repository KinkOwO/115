#!/usr/bin/env python3
"""df33: does an inbound 1565 frame ever reach sub_1444E8D00?

df32 closed: holder+112's constructor default is 1, so the 普通伤害 tab's 解除
sends id 1 and id 1 is also sub_1447EBED0's built-in default-font path. So the
value we echo is right, yet the live run (2026-09-27 05:51) shows the preview and
the in-dungeon numbers keeping the applied font => the echo's effect never
happened.

df7's table says 1565 is registered twice: kind=noti -> sub_14399DD70 (an
unrelated subsystem: reads 8 bytes, posts UI event 1636) and kind=cmd ->
sub_1444E8D00. Two things were never proven: (a) which registry the receive loop
consults, and (b) why sub_1444E8D00 bails out when its second argument is 0.
This round only reads: registrar disassembly at the recorded sites (the 4th
`flags` argument was never captured), the two registrars, the receive loop and
the registry dispatcher.
"""
import os
import re
import ida_bytes
import ida_funcs
import ida_hexrays
import ida_name
import idaapi
import idautils
import idc

OUT = r"D:\115us\analysis\dumps\skin-noti"
ROUND = "df33"

SITES = {
    "noti1545": 0x1444E8586,
    "noti1546": 0x1444E8550,
    "noti1547": 0x1444E856B,
    "cmd1565": 0x1444E85A1,
    "noti1565": 0x14399D604,
    "noti1672": 0x1444E85BC,
}
VAS = {
    "reg_cmd": 0x14599D450,
    "reg_noti": 0x14599D5D0,
    "recv_loop": 0x146D74A80,
    "dispatcher": 0x1459A1BB0,
    "cmd1565_handler": 0x1444E8D00,
}


def dump(tag, text):
    name = "%s_%s.c" % (ROUND, re.sub(r"\W+", "_", tag))
    path = os.path.join(OUT, name)
    with open(path, "w", encoding="utf-8") as h:
        h.write("// %s\n\n%s\n" % (tag, text))
    print("wrote %s (%d bytes)" % (name, len(text)))
    return name


def body_of(ea):
    try:
        return str(ida_hexrays.decompile(ea))
    except Exception as exc:
        return "// decompile failed: %s" % exc


def disasm_range(start, count):
    lines = []
    ea = start
    for _ in range(count):
        if ea == idaapi.BADADDR:
            break
        lines.append("%s: %s" % (hex(ea), idc.GetDisasm(ea)))
        ea = idc.next_head(ea, ea + 16)
    return "\n".join(lines)


import idaapi  # noqa: E402

print("=== 1. call-site constants (registrar arguments incl. the 4th flag) ===")
for tag, site in sorted(SITES.items()):
    fn = ida_funcs.get_func(site)
    base = fn.start_ea if fn else site - 0x40
    text = disasm_range(base, 90)
    dump("site_%s_0x%x" % (tag, site), text)
    print("--- %s @ %s ---" % (tag, hex(site)))
    print(text)

print("=== 2. store offsets (registrars, receive loop, dispatcher) ===")
for tag, ea in sorted(VAS.items()):
    name = ida_name.get_name(ea)
    text = body_of(ea)
    dump("caller_%s_0x%x" % (tag, ea), "// %s\n%s" % (name, text))
    print("--- %s (%s) ---" % (tag, name))
    print(text[:4000])

print("=== 3. parent/child calls ===")
for tag, ea in (("dispatcher", 0x1459A1BB0), ("recv_loop", 0x146D74A80),
                ("cmd1565_handler", 0x1444E8D00)):
    refs = []
    for xr in idautils.CodeRefsTo(ea, 0):
        frm = getattr(xr, "frm", xr)
        refs.append("%s <- %s (%s)" % (hex(frm), idc.GetDisasm(frm),
                                       ida_funcs.get_func(frm) and
                                       idc.get_func_name(frm.start_ea)))
    text = "%d callers\n%s" % (len(refs), "\n".join(refs[:60]))
    dump("xrefs_to_%s_0x%x" % (tag, ea), text)
    print("--- %s ---" % tag)
    print(text)

# where the two registry containers live: readers of qword_14E66C090
gl = idc.get_name_ea_simple("qword_14E66C090")
print("=== 3b. readers of qword_14E66C090 (%s) ===" % hex(gl))
readers = []
for xr in idautils.DataRefsTo(gl):
    f = ida_funcs.get_func(xr)
    if f:
        readers.append("%s %s" % (hex(xr), idc.get_func_name(f.start_ea)))
seen = []
for r in readers:
    fn = r.split()[1]
    if fn not in seen:
        seen.append(fn)
print("%d refs, %d distinct functions" % (len(readers), len(seen)))
print("\n".join(sorted(seen)[:80]))
dump("globals_cmd_registry_readers", "\n".join(readers))
print("=== df33 done ===")

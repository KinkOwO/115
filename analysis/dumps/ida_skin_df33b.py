#!/usr/bin/env python3
"""df33b: which registry does each flag branch consult, and with what a2?

df33 closed three things. (1) The two registrars write two *different* lazily
built tables: CMD -> qword_14E6836F8 via sub_1459A2FB0(reg,id,handler),
NOTI -> qword_14E683700 via sub_1459A3DD0(reg,id,handler,flags). (2) The router
sub_1459A1BB0 branches on the frame's first byte (*a3): 0 -> sub_14599D380,
1 -> sub_14599D200, anything else -> telemetry error, no dispatch. (3) The 1565
CMD handler sub_1444E8D00 returns early (sub_1444EC790, a localized toast) when
its second argument is 0.

This round reads the two branch functions to see which table each looks the id
up in and how it calls the handler, plus the two insert functions so the
tables' record layout is known.
"""
import os
import re
import ida_hexrays
import ida_name
import idaapi
import idautils
import idc

OUT = r"D:\115us\analysis\dumps\skin-noti"
ROUND = "df33b"

TARGETS = {
    "branch_flag0_14599d380": 0x14599D380,
    "branch_flag1_14599d200": 0x14599D200,
    "insert_cmd_1459a2fb0": 0x1459A2FB0,
    "insert_noti_1459a3dd0": 0x1459A3DD0,
    "handler_1444ee820": 0x1444EE820,
}


def dump(tag, text):
    name = "%s_%s.c" % (ROUND, re.sub(r"\W+", "_", tag))
    path = os.path.join(OUT, name)
    with open(path, "w", encoding="utf-8") as h:
        h.write("// %s\n\n%s\n" % (tag, text))
    print("wrote %s (%d bytes)" % (name, len(text)))


def body_of(ea):
    try:
        return str(ida_hexrays.decompile(ea))
    except Exception as exc:
        return "// decompile failed: %s" % exc


print("=== 1. call-site constants ===")
for tag, ea in sorted(TARGETS.items()):
    text = body_of(ea)
    dump("fn_" + tag, "// %s\n%s" % (ida_name.get_name(ea), text))
    print("--- %s ---" % tag)
    print(text[:6000])

print("=== 2. store offsets (globals touched by the two tables) ===")
for gname in ("qword_14E6836F8", "qword_14E683700"):
    ea = idc.get_name_ea_simple(gname)
    refs = []
    for xr in idautils.DataRefsTo(ea):
        f = idc.get_func_name(xr)
        refs.append("%s in %s: %s" % (hex(gname), hex(xr), f))
    text = "\n".join(refs)
    dump("readers_" + gname, text)
    print("--- %s: %d refs ---" % (gname, len(refs)))
    print(text)

print("=== 3. parent/child calls ===")
for fn, ea in (("branch_flag0", 0x14599D380), ("branch_flag1", 0x14599D200)):
    outs = []
    f = idaapi.get_func(ea)
    if not f:
        continue
    head = f.start_ea
    while head != f.end_ea:
        if idc.print_insn_mnem(head) == "call":
            t = idc.print_operand(head, 0)
            outs.append("%s -> %s %s" % (hex(head), t, idc.get_func_name(int(t, 16)) if t.startswith("sub_") else ""))
        head = idc.next_head(head, f.end_ea)
    text = "\n".join(outs)
    dump("calls_from_" + fn, text)
    print("--- %s: %d calls ---" % (fn, len(outs)))
    print(text)
print("=== df33b done ===")

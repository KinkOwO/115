#!/usr/bin/env python3
"""df33c: last link — does the CMD table lookup call the registered handler?

df33b closed: the router sub_1459A1BB0 branches on the frame's first byte.
flag 0 -> sub_14599D380 -> NOTI table qword_14E683700 (this is where a
server-sent 1565 currently lands: sub_14399DD70, an unrelated subsystem that
reads 8 bytes and posts UI event 1636). flag 1 -> sub_14599D200, which consumes
a u8 status byte from the body (and a u16 code only when status == 0) and then
calls sub_1459A2D70(CMD table qword_14E6836F8, id, status, code). The 1565 CMD
handler sub_1444E8D00(a1, a2, a3) matches that shape: `if (!a2) return
sub_1444EC790(a3)` is a failure toast, and the live-confirmed kind=1 acks in this
server all start their body with the byte 1 (protocol.BoosterOpenSuccess).

This round only dumps sub_1459A2D70 to prove the table lookup really invokes the
handler with (status, code) and does not reset the read cursor.
"""
import os
import re
import ida_hexrays
import ida_name
import idc

OUT = r"D:\115us\analysis\dumps\skin-noti"
ROUND = "df33c"

TARGETS = {
    "cmd_dispatch_1459a2d70": 0x1459A2D70,
    "noti_dispatch_1459a3bb0": 0x1459A3BB0,
    "noti_call_1456b8ab0": 0x1456B8AB0,
}


def dump(tag, text):
    name = "%s_%s.c" % (ROUND, re.sub(r"\W+", "_", tag))
    with open(os.path.join(OUT, name), "w", encoding="utf-8") as h:
        h.write("// %s\n\n%s\n" % (tag, text))
    print("wrote %s (%d bytes)" % (name, len(text)))


print("=== 1. call-site constants ===")
for tag, ea in sorted(TARGETS.items()):
    try:
        text = str(ida_hexrays.decompile(ea))
    except Exception as exc:
        text = "// decompile failed: %s" % exc
    dump("fn_" + tag, "// %s\n%s" % (ida_name.get_name(ea), text))
    print("--- %s ---" % tag)
    print(text[:5000])

print("=== 2. store offsets (raw disasm of the CMD dispatch) ===")
ea = 0x1459A2D70
lines = []
for _ in range(70):
    if ea == idaapi.BADADDR:
        break
    lines.append("%s: %s" % (hex(ea), idc.GetDisasm(ea)))
    ea = idc.next_head(ea, ea + 16)
dump("disasm_cmd_dispatch_1459a2d70", "\n".join(lines))
print("\n".join(lines))

print("=== 3. parent/child calls ===")
refs = []
for xr in idautils.CodeRefsTo(0x1459A2D70, 0):
    frm = getattr(xr, "frm", xr)
    refs.append("%s: %s" % (hex(frm), idc.GetDisasm(frm)))
text = "%d callers\n%s" % (len(refs), "\n".join(refs[:40]))
dump("xrefs_to_cmd_dispatch", text)
print(text)
print("=== df33c done ===")

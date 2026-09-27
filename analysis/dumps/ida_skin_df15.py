#!/usr/bin/env python3
"""df15: read the packet dispatch slots for the skin-cargo family.

analysis/dumps/opcode_table_detailed.json gives every CMD/NOTI a "dispatch slot
VA". NOTI1231 DAMAGE_FONT_SKIN_LIST and CMD1282 SELECT_DAMAGE_FONT_SKIN have
zero literal-id registrations (df4/df5 evidence), so the slot table itself is
the next truth source: dump what lives at each slot, who writes it, and whether
the neighbouring slots point at code we can decompile.
"""
import ida_bytes
import ida_funcs
import ida_name
import ida_segment
import idautils
import idc

SLOTS = {
    "noti1231 DAMAGE_FONT_SKIN_LIST": 0x14EF35B28,
    "noti1545 SKIN_CARGO_INFO": 0x14EF364F8,
    "noti1546 SELECT_SKIN_LIST": 0x14EF36500,
    "noti1547 RECENT_ADD_SKIN_LIST": 0x14EF36508,
    "cmd1282 SELECT_DAMAGE_FONT_SKIN": 0x14EF3B770,
    "cmd1565 SELECT_SKIN": 0x14EF3C048,
    "cmd857 OPEN_AURA_SKIN_SLOT": 0x14EF3AA28,
    "noti2641 FAVORITE_SKIN": 0x14EF38738,
    "name1231": 0x14B010440,
    "name1545": 0x14B019800,
}

for tag, ea in sorted(SLOTS.items(), key=lambda kv: kv[1]):
    seg = ida_segment.getseg(ea)
    segname = ida_segment.get_segm_name(seg) if seg else "?"
    words = []
    for i in range(6):
        v = ida_bytes.get_qword(ea + 8 * i)
        nm = ida_name.get_name(v) if v else ""
        f = ida_funcs.get_func(v) if v else None
        words.append("q%d=%s%s" % (i, hex(v), "<=%s" % nm if (nm or f) else ""))
    xb = list(idautils.XrefsTo(ea, 0))
    print("%-36s %s @0x%X  %s" % (tag, segname, ea, " ".join(words)))
    for x in xb[:8]:
        site = idc.generate_disasm_line(x.frm, 0) or "?"
        print("     xref from 0x%X type=%s : %s" % (x.frm, x.type, site.split("\n")[0]))

print("")
print("=== who reads the string VAs (registration sites) ===")
for tag, ea in (("str1231", 0x14B010440), ("str1545", 0x14B019800), ("str1282", 0x14B05A790)):
    for x in idautils.XrefsTo(ea, 0):
        f = ida_funcs.get_func(x.frm)
        print("  %s <- 0x%X in %s : %s" % (tag, x.frm, ida_name.get_name(f.start_ea) if f else "?",
                                           idc.generate_disasm_line(x.frm, 0)))

print("")
print("=== slot table neighbourhood layout (noti1545 +-3 slots) ===")
base = 0x14EF364F8
for off in range(-64, 80, 8):
    ea = base + off
    v = ida_bytes.get_qword(ea)
    print("  0x%X -> 0x%X %s" % (ea, v, ida_name.get_name(v) if v else ""))

idc.qexit(0)

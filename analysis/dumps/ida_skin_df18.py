#!/usr/bin/env python3
"""df18: find the tab/category constant of the damage-font panel class.

df17 showed 0x14A230DF8 is not an array index: `lea rax, off_14A230DE8` inside
`sub_1441B8E90` means off_14A230DE8 is a vtable base and the damage-font panel
builder sub_1441CBDD0 sits at vtable slot +16. So sub_1441B8E90 constructs the
panel object, and whatever category constant it stores is the cargo page/tab the
damage-font list uses. Dump both panel constructors plus every caller of the
constructor with the immediate each caller supplies.
"""
import ida_bytes
import ida_funcs
import ida_hexrays
import ida_name
import idautils
import idc

CTOR_A = 0x1441B8E90
CTOR_B = 0x1441B9A20
VTABLE = 0x14A230DE8


def dump(tag, ea):
    print("\n---- %s 0x%X (%s) ----" % (tag, ea, ida_name.get_name(ea) or "?"))
    try:
        print(ida_hexrays.decompile(ea))
    except Exception as exc:
        print("  <failed %s>" % exc)


print("=== vtable slots starting at off_14%X ===" % VTABLE)
for i in range(12):
    ea = VTABLE + 8 * i
    v = ida_bytes.get_qword(ea)
    print("  [%2d] 0x%X -> 0x%-12X %s" % (i, ea, v, ida_name.get_name(v) or "?"))

dump("panel_ctor_a", CTOR_A)
dump("panel_ctor_b", CTOR_B)

print("\n=== callers of the panel constructors (with the immediates they pass) ===")
for target in (CTOR_A, CTOR_B):
    for x in idautils.XrefsTo(target, 0):
        f = ida_funcs.get_func(x.frm)
        print("  0x%X <- %s 0x%X : %s" % (x.frm, ida_name.get_name(f.start_ea) if f else "?",
                                          f.start_ea if f else 0,
                                          (idc.generate_disasm_line(x.frm, 0) or "").split("\n")[0]))
        cur = x.frm
        for _ in range(14):
            cur = idc.prev_head(cur)
            if cur == idc.BADADDR:
                break
            line = (idc.generate_disasm_line(cur, 0) or "").split("\n")[0]
            if any(t in line for t in ("mov     r", "mov     d", "lea", "call")):
                print("        %s" % line)

idc.qexit(0)

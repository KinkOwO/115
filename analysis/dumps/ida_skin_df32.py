#!/usr/bin/env python3
"""df32: what is skin id 1 to the normal-damage tab?

df31 closed: the panel's two tabs each own a vector at mgr+296[category]; CMD1565's
first u32 is that category (2 normal / 6 cumulative); the 88-byte echo's case 2 calls
sub_1447EF2F0(holder, ids[0]) with no ownership check, which is why it was built as the
normal tab's only complete unequip path.

Live 2026-09-27 05:28 (roles_persist_..._052804_605462_next37) contradicts one half of
that model: the cumulative tab's 解除 sends ids[0]=99999999, but the normal tab's 解除
sends ids[0]=1, seven times, always with category 2 and never 99999999. The server
refuses it ("damage font skin 1 is not owned"), so no echo and no reset frame ever left.

Hypothesis: 1 is that tab's own built-in default font, i.e. the value the client itself
puts in holder+112 before anything is applied (and 99999999 is the counterpart in
holder+232). If the constructors initialize those two cells that way, answering id 1 is
exactly "back to default" and needs no invented semantics.

Section 1 decompiles the two small constructors (decisive). Section 2 lists who else
calls the two setters, in case the button has a local path. Section 3 is a best-effort
disasm sweep of sub_146EA0BE0 call sites for the 88-byte block, to locate the sender.
No known-large function is decompiled.
"""
import os
import re

import ida_hexrays
import ida_name
import idautils
idc = __import__("idc")
idaapi = __import__("idaapi")

OUT = r"D:\115us\analysis\dumps\skin-noti"

HOLDER_CTOR = 0x1447E41D0   # 336-byte holder qword_14E63AE60
OBJ408_CTOR = 0x1447E40D0   # the 408-byte normal-damage object (window+3680)
READER_BULK = 0x146EA0BE0   # sub_146EA0BE0(buf, n)
SET_NORMAL = 0x1447EF2F0    # writes holder+112
SET_CUMUL = 0x142581F20     # writes holder+232


def dump(tag, text):
    name = "df32_%s.c" % re.sub(r"\W+", "_", tag)
    with open(os.path.join(OUT, name), "w", encoding="utf-8") as h:
        h.write("// %s\n\n%s\n" % (tag, text))
    print("wrote %s" % name, flush=True)
    return name


def body_of(ea):
    try:
        return str(ida_hexrays.decompile(ea))
    except Exception as exc:  # thunks and data throw; keep enumerating
        return "// decompile failed: %s" % exc


def mnem(ea):
    try:
        return idc.print_insn_mnem(ea)
    except Exception:
        return ""


print("=== 1. constructor defaults: holder+112 and holder+232 ===", flush=True)
report = []
for tag, ea in (("holder_ctor_sub_1447E41D0", HOLDER_CTOR),
                ("obj408_ctor_sub_1447E40D0", OBJ408_CTOR)):
    text = body_of(ea)
    dump(tag, text)
    stores = re.findall(r"\(\*.*?\+\s*(112|232)\)\)\s*=\s*([^;]{1,48});", text)
    report.append("%s @%s alloc=%s" % (tag, hex(ea), re.findall(r"sub_146E8BA20\((\d+)\)", text)))
    report.append("  +112/+232 stores: %s" % (stores or "none"))
    report.append("  99999999 in body: %s" % ("99999999" in text or "5F5E100" in text.upper()))
print("\n".join(report), flush=True)
dump("ctor_summary", "\n".join(report))

print("=== 2. callers of the two font setters ===", flush=True)
out2 = []
for tag, ea in (("set_normal_holder112", SET_NORMAL), ("set_cumulative_holder232", SET_CUMUL)):
    callers = [xr.frm for xr in idautils.CodeRefsTo(ea, 0) if mnem(xr.frm) == "call"]
    named = []
    for c in callers:
        f = idc.get_func_attr(c, idc.FUNCATTR_START)
        owner = "?" if f == idaapi.BADADDR else "%s(%s)" % (hex(f), ida_name.get_ea_name(f))
        named.append("  call @%s from %s" % (hex(c), owner))
    out2.append("%s (%s): %d callers\n%s" % (tag, hex(ea), len(callers), "\n".join(named)))
    print(out2[-1], flush=True)
dump("setters_callers", "\n\n".join(out2))

print("=== 3. sub_146EA0BE0 call sites and their size operand ===", flush=True)
out3 = []
for xr in idautils.CodeRefsTo(READER_BULK, 0):
    frm = xr.frm
    if mnem(frm) != "call":
        continue
    size = None
    ea = frm
    for _ in range(16):
        ea = idc.prev_head(ea, ea - 0x40)
        if ea == idaapi.BADADDR or mnem(ea) == "call":
            break
        if mnem(ea) != "mov" or idc.get_operand_type(ea, 1) != idc.o_imm:
            continue
        dest = idc.print_operand(ea, 0).upper()
        if dest.endswith("DX") or dest in ("EDX", "RDX"):
            size = idc.get_operand_value(ea, 1)
            break
    owner = idc.get_func_attr(frm, idc.FUNCATTR_START)
    out3.append("%s size=%s from %s(%s)" % (
        hex(frm), size, hex(owner) if owner != idaapi.BADADDR else "?",
        ida_name.get_ea_name(owner) if owner != idaapi.BADADDR else "?"))
    print(out3[-1], flush=True)
dump("readsites", "\n".join(out3))

print("=== done ===", flush=True)
os._exit(0)

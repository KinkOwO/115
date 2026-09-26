#!/usr/bin/env python3
"""df24: who sends CMD1565, what does its 88-byte body hold, and which category is the damage font?

df23 closed the render half: the damage-font panel refills from owned page 2, so
NOTI1545 page 2 is the cargo page. Live 2026-09-27 then confirmed the grid fills, and
the click on 应用 emits CMD1565, 88 plain bytes: `u32 2, u32 0, u32 12, 0...` (12 is
the skin id the server registered for template 10305398) and, from the 边框 tab,
`u32 1, u32 0, u32 100000, ...`. Nothing changed on screen, so the client waits for
the server.

The client's own CMD1565 receive path is already dumped: registrar site 0x1444E85A1
maps opcode 1565 -> sub_1444E8D00(mgr, char ok, u16 code), which on ok shows the
error text sub_1444EC790(code) and otherwise calls sub_1444EE820, which bulk-reads 88
bytes as `u32 category, u32 code, u32 ids[20]` and switches over the same category set
as the NOTI1546 reader sub_1444EECA0 (0,1,2,3,4,6,7,8,9).

So the open question is which of those two namespaces the request's leading u32 uses:
EECA0 case 6 validates ownership in page 2 and applies via sub_142581F20, while case 2
does no page-2 check and applies via sub_1447EF2F0. df24 finds the sender (callers of
the CMD builder that pass 0x61D, plus sub_1444E8ED0 which EECA0 case 0 calls with a
skin id and with the 80000 sentinel), and decompiles the two setters so the reply
category is proven rather than assumed.
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
CMD_BUILDER = 0x140069BB0
SENDER_CANDIDATE = 0x1444E8ED0
OPCODE_1565 = 0x61D
PANEL_VTABLE = 0x14A230DE8
READER_1546 = 0x1444EECA0


def body_of(ea):
    try:
        return str(ida_hexrays.decompile(ea))
    except Exception:
        return ""


def dump(tag, text):
    name = "df24_%s.c" % re.sub(r"\W+", "_", tag)
    with open(os.path.join(OUT, name), "w", encoding="utf-8") as handle:
        handle.write("// %s\n\n%s\n" % (tag, text))
    return name


def callers(ea, label):
    """Call sites of `ea` with the immediates loaded into the argument registers."""
    out = []
    for xr in idautils.XrefsTo(ea, 0):
        if idc.print_insn_mnem(xr.frm) != "call":
            continue
        fn = ida_funcs.get_func(xr.frm)
        head, regs = xr.frm, {}
        for _ in range(14):
            head = idc.prev_head(head)
            if head == 0xFFFFFFFFFFFFFFFF:
                break
            line = (idc.GetDisasm(head) or "").split("\n")[0]
            m = re.match(r"mov\s+(e[a-d]x|[a-d]l|si|di|r8d)\s*,\s*(\w+)", line)
            if m:
                regs.setdefault(m.group(1), m.group(2))
            if re.match(r"xor\s+(e[a-d]x|[a-d]l)\s*,", line):
                regs.setdefault(line.split()[1].strip(), "0")
        out.append((xr.frm, ida_name.get_name(fn.start_ea) if fn else "?", dict(regs)))
    print("  %d call sites of %s" % (len(out), label))
    for site, name, regs in out:
        print("      0x%X %-18s %s" % (site, name, regs))
    return out


print("=== 1. call sites of the CMD builder, keeping those that pass opcode 1565 ===")
sites = callers(CMD_BUILDER, "sub_140069BB0 (cmd builder)")
hot = [s for s in sites if any(v.lower() in ("61dh", "0x61d", "1565") for v in s[2].values())]
print("  sites passing 0x61D: %s" % [hex(x[0]) for x in hot])

print("\n=== 2. the reader/sender/panel closure, looking for the builder call ===")
seeds = [READER_1546, 0x1444EE820, SENDER_CANDIDATE, 0x1441E3510, 0x1441ECBC0, 0x1444E8D00]
closures, seen, depth = set(), set(), 3
frontier = list(seeds)
while depth and frontier:
    nxt = []
    for ea in frontier:
        if ea in seen:
            continue
        seen.add(ea)
        try:
            nxt.extend(idautils.FunctionCalls(ea) or [])
        except Exception:
            pass
    frontier = nxt
    closures.update(nxt)
    depth -= 1
    if len(closures) > 400:
        break
print("  scanning %d functions in the closure" % len(closures))
senders = []
for ea in sorted(closures):
    body = body_of(ea)
    if not body:
        continue
    if "sub_140069BB0" in body or "1565" in body:
        senders.append(ea)
        dump("hit_" + (ida_name.get_name(ea) or hex(ea)), body)
print("  functions in the closure mentioning the builder or 1565: %s" % [hex(x) for x in senders])

print("\n=== 3. the sender candidate and its call sites with the id constant ===")
dump("sender_" + hex(SENDER_CANDIDATE), body_of(SENDER_CANDIDATE))
callers(SENDER_CANDIDATE, "sub_1444E8ED0")

print("\n=== 4. which setter changes the damage numbers ===")
for ea in (0x142581F20, 0x1447EF2F0, 0x1447EF380, 0x1447EF3B0, 0x143C61150):
    body = body_of(ea)
    nm = ida_name.get_name(ea) or hex(ea)
    fn = ida_funcs.get_func(ea)
    n_callers = sum(1 for xr in idautils.XrefsTo(ea, 0) if idc.print_insn_mnem(xr.frm) == "call")
    print("  %s %-14s callers=%-4d callees=%s" % (
        hex(ea), nm, n_callers, [hex(c) for c in idautils.FunctionCalls(ea)][:8] if fn else []))
    print("      %s" % (body.split("\n")[0] if body else "?")[:110])
    dump("set_" + nm, body)

print("\n=== 5. damage-font panel vtable slots, and which of them reach the sender ===")
for slot in range(12):
    ptr = ida_bytes.get_qword(PANEL_VTABLE + 8 * slot)
    if not ptr:
        continue
    body = body_of(ptr)
    reach = "sub_1444E8ED0" in body
    print("  slot %-2d 0x%X %-16s sender=%-5s len=%d" % (slot, ptr, ida_name.get_name(ptr) or "?", reach, len(body)))
    if reach:
        dump("vslot_%d" % slot, body)

idc.qexit(0)

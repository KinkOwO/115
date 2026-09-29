#!/usr/bin/env python3
"""df35: what fills the static skin registry record's +8 (family class) and +12 (subtype)?

df34 closed: qword_14E683BF8 has 33 refs in 29 functions; inside the skin-cargo cluster
0x1444E8000-0x1444F2200 every one of them is a READ through the lookup helper
sub_140283D60(global, id, 1) (df34.log section 1/2). The only ref sites that are not
reads-through-the-lookup are three neighbouring functions in the data-load cluster:
sub_146D7BDC0 (2269B, 1 callee), sub_146D7C710 (1529B, 6 callees, 'jumptable case 64'),
sub_146D7CD10 (2177B, 1 callee). df34's candidate cap (first 15, sorted ascending) never
reached them, so this round decompiles exactly those three.

Why it matters: the panel grids prove page==class (df22_ui_1441E3F40 keeps rec+8==0 and
drops ids 20000/50000/60000/80000; df22_ui_1441E4180 keeps rec+8==1 and drops
30000/100000), and the party-frame grid filters rec+12 against its tab mode. So the int
stored at +8/+12 decides which NOTI1545 page a family must be pushed to and which
NOTI1546 slot a click belongs to. The PVF only has the strings.
"""
import os, re, ida_hexrays, ida_name, idautils, idc

OUT = r"D:\115us\analysis\dumps\skin-noti"
TARGETS = [0x146D7BDC0, 0x146D7C710, 0x146D7CD10]


def body_of(ea):
    try:
        return str(ida_hexrays.decompile(ea))
    except Exception as e:
        return ""


def dump(tag, text):
    name = "df35_%s.c" % re.sub(r"\W+", "_", tag)
    with open(os.path.join(OUT, name), "w", encoding="utf-8") as h:
        h.write("// %s\n\n%s\n" % (tag, text))
    return name


print("=== 1. callers of the three registry ref sites ===")
for t in TARGETS:
    callers = []
    for xr in idautils.XrefsTo(t, 0):
        if idc.print_insn_mnem(xr.frm) == "call":
            callers.append("%s@%s" % (hex(xr.frm), ida_name.get_name(idc.get_func_attr(xr.frm, idc.FUNCATTR_START)) or "-"))
    print("target=%s(%s) call_sites=%d %s" % (hex(t), ida_name.get_name(t), len(callers), " ".join(callers[:8])))

print("=== 2. decompile the three, report +8/+12 stores and callees ===")
for t in TARGETS:
    b = body_of(t)
    if not b:
        print("target=%s decompile failed" % hex(t))
        continue
    n = dump("loadfn_0x%x" % t, b)
    stores = re.findall(r"\+\s*(8|12)\)\s*=\s*([^;]{1,60});", b)
    calls = sorted(set(int(x[4:], 16) for x in re.findall(r"sub_(1[0-9A-Fa-f]{7})", b)))
    print("target=%s dumped=%s len=%d +8/+12 stores=%d" % (hex(t), n, len(b), len(stores)))
    for off, rhs in stores[:20]:
        print("   +%-3s = %s" % (off, rhs.strip()))
    print("   callees=%s" % " ".join(ida_name.get_name(c) or hex(c) for c in calls[:20]))

print("=== 3. string-data referenced by them (type labels are not plaintext) ===")
for t in TARGETS:
    seen = set()
    for h in idautils.FuncItems(t):
        for xr in idautils.DataRefs(h):
            sz = ida_bytes_get = idc.get_str_type(xr)
            txt = idc.get_strlit_contents(xr, -1, 0)
            if txt:
                s = txt.decode("utf-8", "replace")
                if s not in seen:
                    seen.add(s)
                    print("0x%x %s -> %r" % (t, hex(xr), s[:80]))
    print("target=%s strings=%d" % (hex(t), len(seen)))
print("=== done ===")

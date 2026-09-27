#!/usr/bin/env python3
"""df36: is 觉醒插图's "random" a server-visible switch, or just the size of mgr+296[1]?

Closed before this round (all in analysis/dumps/skin-noti):
  * sub_1444EA8A0 (df26_ebc10caller_sub_1444EA8A0_0x1444ea8a0.c): mgr+1120 = 30000, then if
    sub_1444EBC10(mgr,&v,1) is non-empty, mgr+1120 = *vec[RNG % size].  So a one-element
    vector is deterministic and a two-or-more-element vector IS the random draw.
  * NOTI1546 category 1 (df13_noti1546_body_1444eeca0.c, LABEL_86 / LABEL_155): the frame's
    FIRST list replaces mgr+296[1]; the SECOND list is assigned to mgr+1152 and mgr+1176.
  * CMD1565 echo case 1 (df23_sub_1444EE820.c:81) refuses to store a slot whose static
    registry record has +8==1 && +12==3 (the second-awakening class).
  * live 2026-09-27 23:24 session (13 C2S 1565): every category-1 body carries the family's
    default cells (30000 and/or 100000) next to the clicked skin, and the server's stored
    list therefore grows (one body held 100000 twice).
  * sub_1444F1410 (df26_ebc10caller_sub_1444F1410_0x1444f1410.c) builds a 20-slot u32 body
    out of mgr+1176 (slots 0..9) and mgr+1128 (slots 10..19) and hands it to
    sub_1444F1090(mgr, 1, vec) - the only 20-slot composer seen so far.

Open questions this round:
  1. is sub_1444F1090 the CMD1565 sender (which opcode constant does it use), and who calls
     it?  A panel 应用 handler would prove the request body is the panel's own cell set.
  2. does ANY function read mgr+1152 / mgr+1176, or is that list write-only?
  3. who calls sub_1444EA8A0 - is the random draw gated by a flag anywhere?
  4. who calls sub_1444F1410.
"""
import os, re, ida_hexrays, ida_name, idautils, idc

OUT = r"D:\115us\analysis\dumps\skin-noti"
ROUND = "df36"

F1090 = 0x1444F1090
F1410 = 0x1444F1410
EA8A0 = 0x1444EA8A0
CLUSTER_LO = 0x1444E8000
CLUSTER_HI = 0x1444F2200


def body_of(ea):
    try:
        return str(ida_hexrays.decompile(ea))
    except Exception:
        return ""


def dump(tag, text):
    name = "%s_%s.c" % (ROUND, re.sub(r"\W+", "_", tag))
    path = os.path.join(OUT, name)
    with open(path, "w", encoding="utf-8") as h:
        h.write("// %s\n\n%s\n" % (tag, text))
    return name


def callers_of(ea):
    out = []
    for xr in idautils.XrefsTo(ea, 0):
        if idc.print_insn_mnem(xr.frm) == "call":
            start = idc.get_func_attr(xr.frm, idc.FUNCATTR_START)
            out.append(start if start != idc.BADADDR else xr.frm)
    seen, uniq = set(), []
    for c in out:
        if c not in seen:
            seen.add(c)
            uniq.append(c)
    return uniq


print("=== 1. call-site constants ===")
for ea, label in ((F1090, "F1090"), (F1410, "F1410"), (EA8A0, "EA8A0")):
    cs = callers_of(ea)
    print("%s 0x%X callers (%d): %s" % (label, ea, len(cs),
          ", ".join("0x%X(%s)" % (c, ida_name.get_name(c)) for c in cs)))

print("=== 2. store offsets ===")
# who touches mgr+1152 / mgr+1176 inside the skin-manager cluster, and is it a read?
hits = {}
for f in idautils.Functions():
    if not (CLUSTER_LO <= f < CLUSTER_HI):
        continue
    b = body_of(f)
    if not b:
        continue
    found = re.findall(r"\+\s*(1128|1136|1152|1160|1176|1184)\)", b)
    if found:
        hits[f] = sorted(set(found))
for f, offs in sorted(hits.items()):
    print("0x%X %s offsets %s" % (f, ida_name.get_name(f), ",".join(offs)))

print("=== 3. decompile the anchors and their callers ===")
dump("fn_f1090_0x1444f1090", body_of(F1090))
for c in callers_of(F1090):
    dump("caller_f1090_0x%x" % c, body_of(c))
dump("fn_f1410_0x1444f1410", body_of(F1410))
for c in callers_of(F1410):
    dump("caller_f1410_0x%x" % c, body_of(c))
dump("fn_ea8a0_0x1444ea8a0", body_of(EA8A0))
for c in callers_of(EA8A0):
    dump("caller_ea8a0_0x%x" % c, body_of(c))
print("=== done ===")
ida_hexrays.mark_cfunc_dirty(F1090, True)

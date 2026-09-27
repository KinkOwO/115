#!/usr/bin/env python3
"""df37: does the dungeon-time second-awakening cutscene id come from mgr+1152, i.e. the
list NOTI1546 category 1 fills from its SECOND list (which this server always sends empty)?

Closed before this round (all in analysis/dumps/skin-noti):
  * df36 caller dump df36_caller_ea8a0_0x145d451d0.c: sub_145D451D0(a1,a2) is the function
    that finally writes the cutscene id into the game state (sub_146664340(qword_14E683C68,
    v5)).  It has three sources:
      - sub_145CD39F0(a1,1) == a2  -> v5 = sub_1444EA8A0(mgr)          (first awakening:
                                        mgr+1120 = mgr+296[1][RNG % size])
      - gate sub_145CF7C40(a1,a2), vtable +4832 == 3 and +4848 == 3 and a2 == 247
                                        -> v5 = 100000                (hardcoded default)
      - otherwise                        -> v5 = sub_1444EBAD0(mgr)    (the second-awakening
                                        accessor; mgr = sub_140764510())
  * df36 offset scan: inside the skin-manager cluster only two functions touch mgr+1152 -
    sub_1444EBAD0 (offsets 1152,1160) and sub_1444EECA0 (offsets 1152,1160,1176,1184).
  * df13_noti1546_body_1444eeca0.c:368-371 (case 1LL): the frame's SECOND list is assigned
    to BOTH mgr+1152 and mgr+1176; its FIRST list replaces mgr+296[1] (LABEL_155).
  * df36_fn_f1410_0x1444f1410.c: the category-1 CMD1565 composer puts mgr+1176 into body
    slots 0..9 and mgr+1128 into slots 10..19, so the panel's second-awakening rows are a
    separate list from the first-awakening rows, not extra cells of one choice.
  * server side today: internal/game/protocol/skin_cargo.go SkinSelectionSkillCutscene
    emits `u8 1, u16 n, u32 ids[n], u16 0` - the second list is hardcoded empty on the
    recorded gap "no consumer of those two vectors exists".

Open questions this round:
  1. sub_1444EBAD0's body: with an empty mgr+1152 what does it return (0? 100000?), and
     with a non-empty one does it take element 0 or draw at random?
  2. is mgr+1152 read anywhere else (callers of EBAD0) - and is there a dungeon/enter-town
     snapshot, or is it read live each time?
  3. which functions WRITE mgr+1176 / mgr+1128 (the panel checkbox handlers), so the
     server can tell the 二觉 rows apart from the 一觉 rows inside the 88-byte body.
  4. sub_1444EBB90 / sub_1444EA9D0 - the (mgr, id) variants used by the second half of
     sub_145D451D0.
"""
import os, re, ida_hexrays, ida_name, idautils, idc

OUT = r"D:\115us\analysis\dumps\skin-noti"
ROUND = "df37"

EBAD0 = 0x1444EBAD0
EBB90 = 0x1444EBB90
EA9D0 = 0x1444EA9D0
D451D0 = 0x145D451D0
GETMGR = 0x140764510
G1 = 0x145CD39F0
G2 = 0x145CF7C40
WRITERS = (0x1444E9240, 0x1444E9290, 0x1444E9A40, 0x1444E9AD0,
           0x1444EBF60, 0x1444EBF80, 0x1444EBFA0, 0x1444EBFC0,
           0x1444EC6E0, 0x1444EC710, 0x1444F1B60, 0x1444F1BE0)


def body_of(ea):
    try:
        return str(ida_hexrays.decompile(ea))
    except Exception:
        return ""


def dump(tag, text):
    name = "%s_%s.c" % (ROUND, re.sub(r"\W+", "_", tag))
    with open(os.path.join(OUT, name), "w", encoding="utf-8") as h:
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
for ea, label in ((EBAD0, "EBAD0"), (EBB90, "EBB90"), (EA9D0, "EA9D0"), (D451D0, "D451D0")):
    cs = callers_of(ea)
    print("%s 0x%X callers (%d): %s" % (label, ea, len(cs),
          ", ".join("0x%X(%s)" % (c, ida_name.get_name(c)) for c in cs)))

print("=== 2. store offsets ===")
# which of the cluster writers actually stores into mgr+1152 / mgr+1176 / mgr+1128
for ea in WRITERS:
    b = body_of(ea)
    offs = sorted(set(re.findall(r"\+\s*(1128|1136|1152|1160|1176|1184)\)", b)))
    calls = sorted(set(re.findall(r"(sub_[0-9A-Fa-f]{6,9})", b)))
    print("0x%X %s offsets %s callees %s" % (ea, ida_name.get_name(ea),
          ",".join(offs) or "-", ",".join(calls) or "-"))

print("=== 3. decompile the anchors and their callers ===")
dump("fn_ebad0_0x1444ebad0", body_of(EBAD0))
for c in callers_of(EBAD0):
    dump("caller_ebad0_0x%x" % c, body_of(c))
dump("fn_ebb90_0x1444ebb90", body_of(EBB90))
dump("fn_ea9d0_0x1444ea9d0", body_of(EA9D0))
dump("fn_getmgr_0x140764510", body_of(GETMGR))
for ea in WRITERS:
    dump("writer_0x%x" % ea, body_of(ea))
dump("gate_0x145cd39f0", body_of(G1))
dump("gate_0x145cf7c40", body_of(G2))
print("=== done ===")
ida_hexrays.mark_cfunc_dirty(EBAD0, True)

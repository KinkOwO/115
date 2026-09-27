#!/usr/bin/env python3
"""Find skin packet handlers by intersection: functions that reference the skin
cargo manager singleton qword_14E638F28 AND call one of the packet reader
helpers. The damage-font list (noti1231) is not in the literal-id registrar, but
if it parses into the same manager it must touch the singleton.

Readers: sub_146EA09F0(u8/n), sub_146EA0BA0(u32), sub_146EA0BE0(u64/n),
         sub_146EA1920(u16 count).

Already-known skin handlers/readers are excluded so the output is the *new*
candidate set (damage font, aura, weapon-effect, favorite, etc.).

Run:  idat -A -L<log> -S"ida_skin_df5.py" <work>/DFO.exe.i64
"""
import os
import json
import ida_funcs
import ida_hexrays
import ida_xref
import ida_name
import idaapi
import idc

OUT_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "skin-noti")
os.makedirs(OUT_DIR, exist_ok=True)

MGR = 0x14E638F28
READERS = {0x146EA09F0, 0x146EA0BA0, 0x146EA0BE0, 0x146EA1920}

KNOWN = {
    0x1444ED900, 0x1444ED4F0, 0x1444ED400, 0x1444E8D00, 0x1444EE1C0, 0x1444EDFD0,
    0x1444EFF40, 0x1444EECA0, 0x1444F0270, 0x1444EFA00, 0x1444EC790, 0x1444EE820,
    0x1444E81C0, 0x1444E7F30, 0x1444E8030, 0x1473A1120, 0x1444F1EE0,
}


def funcs_referencing(va):
    out = set()
    for getter, nxt in ((ida_xref.get_first_dref_to, ida_xref.get_next_dref_to),
                        (ida_xref.get_first_cref_to, ida_xref.get_next_cref_to)):
        r = getter(va)
        while r != idaapi.BADADDR:
            f = ida_funcs.get_func(r)
            if f is not None:
                out.add(f.start_ea)
            r = nxt(va, r)
    return out


def calls_reader(fea):
    hit = []
    f = ida_funcs.get_func(fea)
    if f is None:
        return hit
    ea = f.start_ea
    while ea < f.end_ea:
        if idc.print_insn_mnem(ea) == "call":
            tgt = idc.get_operand_value(ea, 0)
            if tgt in READERS:
                hit.append(tgt)
        ea = idc.next_head(ea, f.end_ea)
    return hit


mgr_funcs = funcs_referencing(MGR)
print("functions referencing skin mgr singleton: %d" % len(mgr_funcs))

candidates = []
for fea in sorted(mgr_funcs):
    if fea in KNOWN:
        continue
    rd = calls_reader(fea)
    if rd:
        name = ida_name.get_name(fea) or "sub_%X" % fea
        candidates.append({"func": "0x%X" % fea, "name": name,
                           "readers": ["0x%X" % r for r in sorted(set(rd))]})

print("CANDIDATES (mgr+reader, not already known): %d" % len(candidates))
print(json.dumps(candidates, indent=2))

# Decompile the candidates so we can read their body parsers.
for c in candidates:
    fea = int(c["func"], 16)
    try:
        cf = ida_hexrays.decompile(fea)
    except Exception as exc:
        print("FAIL %s %r" % (c["func"], exc))
        continue
    if cf is None:
        print("NONE %s" % c["func"])
        continue
    text = str(cf)
    p = os.path.join(OUT_DIR, "df5_cand_%s.c" % c["func"].lower())
    open(p, "w", encoding="utf-8").write(text)
    print("OK %s %s lines=%d -> %s" % (c["func"], c["name"], text.count("\n"), p))

json.dump(candidates, open(os.path.join(OUT_DIR, "df5-candidates.json"), "w"), indent=2)
idc.qexit(0)

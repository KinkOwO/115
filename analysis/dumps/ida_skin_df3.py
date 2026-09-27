#!/usr/bin/env python3
"""Locate the damage-font packet dispatch by binary-searching the code for the
id immediates that the registrar scan never saw.

  noti 1231 DAMAGE_FONT_SKIN_LIST       mov edx, 4CFh  -> BA CF 04 00 00
  cmd  1282 SELECT_DAMAGE_FONT_SKIN     mov edx, 502h  -> BA 02 05 00 00
  cmd  857  OPEN_AURA_SKIN_SLOT         mov edx, 359h  -> BA 59 03 00 00
  noti 5056 USE_WEAPON_EFFECT_SKIN_REG  mov edx,13C0h  -> BA C0 13 00 00

For every hit, decompile the containing function and dump it. Also try the
`mov edx, imm`/`cmp reg, imm` encodings via get_operand_value scan limited to
hits returned by find_binary (fast).

Run:  idat -A -L<log> -S"ida_skin_df3.py" <work>/DFO.exe.i64
"""
import os
import json
import ida_funcs
import ida_hexrays
import idaapi
import ida_bytes
import idc

# IDA 9 moved/renamed the binary search API; resolve whatever this build exposes.
_FIND = None
for _mod, _name in ((idaapi, "find_binary"), (ida_bytes, "find_binary"),
                    (ida_bytes, "bin_search"), (idc, "find_binary")):
    _f = getattr(_mod, _name, None)
    if callable(_f):
        _FIND = _f
        break


def find_bin(cur, hi, pat):
    if _FIND is None:
        return idaapi.BADADDR
    try:
        return _FIND(cur, idaapi.SEARCH_DOWN, pat)
    except Exception:
        try:
            return _FIND(cur, hi, pat, 16, idaapi.SEARCH_DOWN)
        except Exception:
            return idaapi.BADADDR


OUT_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "skin-noti")
os.makedirs(OUT_DIR, exist_ok=True)

PATTERNS = {
    1231: ("noti1231_damage_font_skin_list", "BA CF 04 00 00"),
    1282: ("cmd1282_select_damage_font_skin", "BA 02 05 00 00"),
    857: ("cmd857_open_aura_skin_slot", "BA 59 03 00 00"),
    5056: ("noti5056_use_weapon_effect_skin_register", "BA C0 13 00 00"),
}


def decomp(ea):
    f = ida_funcs.get_func(ea)
    start = f.start_ea if f else ea
    try:
        cf = ida_hexrays.decompile(start)
    except Exception:
        return None, start
    return (str(cf) if cf is not None else None), start


results = {}
for tid, (label, pat) in PATTERNS.items():
    hits = []
    lo = idc.get_inf_attr(idc.INF_MIN_EA)
    hi = idc.get_inf_attr(idc.INF_MAX_EA)
    cur = lo
    seen_funcs = set()
    guard = 0
    while cur < hi and guard < 200:
        guard += 1
        nxt = find_bin(cur, hi, pat)
        if nxt == idaapi.BADADDR or nxt >= hi:
            break
        cur = nxt + 1
        f = ida_funcs.get_func(nxt)
        fea = f.start_ea if f else 0
        hits.append({"ea": "0x%X" % nxt, "func": "0x%X" % fea,
                     "mnem": idc.print_insn_mnem(nxt),
                     "disasm": idc.generate_disasm_line(nxt, 0)})
        if fea and fea not in seen_funcs:
            seen_funcs.add(fea)
            text, start = decomp(fea)
            if text:
                p = os.path.join(OUT_DIR, "df3_%s_func_%x.c" % (label, start))
                open(p, "w", encoding="utf-8").write(text)
                print("DECOMP %s func %x lines=%d -> %s" % (label, start, text.count("\n"), p))
    results[tid] = {"label": label, "hits": hits}
    print("HITS %d %s count=%d %s" % (tid, label, len(hits), json.dumps(hits[:8])))

json.dump(results, open(os.path.join(OUT_DIR, "df3-hits.json"), "w"), indent=2)
idc.qexit(0)

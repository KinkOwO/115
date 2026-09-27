#!/usr/bin/env python3
"""Find the handlers for the damage-font packets that the literal-id registrar
scan missed: noti 1231 (0x4CF) DAMAGE_FONT_SKIN_LIST, cmd 1282 (0x502)
SELECT_DAMAGE_FONT_SKIN, cmd 857 (0x359) OPEN_AURA_SKIN_SLOT,
noti 5056 (0x13C0) USE_WEAPON_EFFECT_SKIN_REGISTER.

Two strategies:
 A) Walk every call site of the two registrars, look back up to 12 instructions
    for `mov edx/rdx, <imm>` equal to a target id, capture `lea r8, handler`.
 B) idautils over immediate constants: find any code that materialises the id.

Run:  idat -A -L<log> -S"ida_skin_df2.py" <work>/DFO.exe.i64
"""
import os
import re
import json
import ida_funcs
import ida_hexrays
import idautils
import ida_ua
import idc
import idaapi

NOTI_HELPER = 0x14599D5D0
CMD_HELPER = 0x14599D450
REGISTRARS = (NOTI_HELPER, CMD_HELPER)

TARGETS = {
    0x4CF: "noti1231_damage_font_skin_list",
    0x502: "cmd1282_select_damage_font_skin",
    0x359: "cmd857_open_aura_skin_slot",
    0x13C0: "noti5056_use_weapon_effect_skin_register",
    0x609: "noti1545_skin_cargo_info",   # sanity: should already be known
}

OUT_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "skin-noti")
os.makedirs(OUT_DIR, exist_ok=True)


def decompile_text(ea):
    try:
        cf = ida_hexrays.decompile(ea)
    except Exception as exc:
        return None
    return str(cf) if cf is not None else None


def back_scan(callea):
    """Look back for mov edx, imm (id) and lea r8, off (handler)."""
    pid = None
    handler = None
    ea = callea
    for _ in range(14):
        ea = idc.prev_head(ea)
        if ea == idaapi.BADADDR:
            break
        mnem = idc.print_insn_mnem(ea)
        op0 = idc.print_operand(ea, 0)
        if mnem == "mov" and op0 in ("edx", "rdx", "dx"):
            v = idc.get_operand_value(ea, 1)
            if v in TARGETS:
                pid = v
        if mnem == "lea" and op0 in ("r8", "r8d"):
            handler = idc.get_operand_value(ea, 1)
        if pid is not None and handler is not None:
            break
        if mnem == "call":
            break
    return pid, handler


def main():
    found = {}
    sites = []
    for helper in REGISTRARS:
        for ref in idautils.CodeRefsTo(helper, 0):
            ea = ref if isinstance(ref, int) else ref.frm
            sites.append(ea)
    print("registrar call sites: %d" % len(sites))
    for ea in sites:
        pid, handler = back_scan(ea)
        if pid is not None:
            f = ida_funcs.get_func(ea)
            mgr = f.start_ea if f else 0
            found[pid] = {"manager": "0x%X" % mgr, "handler": "0x%X" % handler if handler else None,
                          "callsite": "0x%X" % ea}
    print("BACKSCAN FOUND %s" % json.dumps({("%s" % k): v for k, v in found.items()}, indent=2))

    # Decompile whatever handlers we resolved.
    for pid, label in TARGETS.items():
        if pid not in found or not found[pid]["handler"]:
            continue
        hea = int(found[pid]["handler"], 16)
        f = ida_funcs.get_func(hea)
        start = f.start_ea if f else hea
        text = decompile_text(start)
        if text is None:
            print("FAIL %s %x" % (label, start))
            continue
        p = os.path.join(OUT_DIR, "df2_handler_%s_%x.c" % (label, start))
        open(p, "w", encoding="utf-8").write(text)
        print("OK %s %x lines=%d -> %s" % (label, start, text.count("\n"), p))

    json.dump({("%d" % k): v for k, v in found.items()},
              open(os.path.join(OUT_DIR, "df2-found.json"), "w"), indent=2)
    idc.qexit(0)


main()

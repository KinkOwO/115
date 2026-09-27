#!/usr/bin/env python3
"""Whole-binary packet registrar table scan.

The skin-cargo ctor registers handlers via:
    lea r8, <handler> ; mov edx, <id> ; mov rcx, <registry> ; call sub_14599D5D0  (NOTI)
    ... same shape with sub_14599D450 (CMD)
NOTI1231 DAMAGE_FONT_SKIN_LIST / CMD1282 SELECT_DAMAGE_FONT_SKIN / CMD857
OPEN_AURA_SKIN_SLOT are NOT in the skin-cargo ctor, so they register elsewhere.

This script walks EVERY call to sub_14599D5D0 and sub_14599D450, backtracks the
edx immediate (packet id) and the r8 lea target (handler), and dumps the full
id -> (handler, kind) table. Then it flags the damage-font ids of interest and
decompiles their handlers.
"""
import os
import ida_funcs
import ida_hexrays
import ida_xref
import ida_name
import idaapi
import idc

OUT_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "skin-noti")
os.makedirs(OUT_DIR, exist_ok=True)

NOTI_REG = 0x14599D5D0
CMD_REG = 0x14599D450
INTEREST = {1231, 1282, 857, 1545, 1546, 1547, 2641, 1565}


def imm_edx_before(call_ea):
    """Backtrack up to 12 heads for `mov edx, imm32` (or mov dx, imm)."""
    ea = call_ea
    for _ in range(12):
        ea = idc.prev_head(ea, ea - 0x60)
        if ea == idaapi.BADADDR:
            break
        m = idc.print_insn_mnem(ea)
        op0 = idc.print_operand(ea, 0)
        if m == "mov" and op0 in ("edx", "rdx", "dx", "edi", "rdi") :
            if idc.get_operand_type(ea, 1) == idc.o_imm:
                return idc.get_operand_value(ea, 1), op0, ea
    return None, None, None


def lea_r8_before(call_ea):
    ea = call_ea
    for _ in range(12):
        ea = idc.prev_head(ea, ea - 0x60)
        if ea == idaapi.BADADDR:
            break
        m = idc.print_insn_mnem(ea)
        op0 = idc.print_operand(ea, 0)
        if m == "lea" and op0 in ("r8", "r8d"):
            if idc.get_operand_type(ea, 1) == idc.o_mem:
                return idc.get_operand_value(ea, 1), ea
    return None, None


table = {}
for reg, kind in ((NOTI_REG, "noti"), (CMD_REG, "cmd")):
    r = ida_xref.get_first_cref_to(reg)
    while r != idaapi.BADADDR:
        pid, opreg, pea = imm_edx_before(r)
        hdl, lea = lea_r8_before(r)
        if pid is not None:
            table.setdefault(pid, []).append(
                {"kind": kind, "handler": ("0x%X" % hdl) if hdl else None,
                 "site": "0x%X" % r, "idreg": opreg})
        r = ida_xref.get_next_cref_to(reg, r)

print("=== registrar table: %d distinct ids ===" % len(table))
for pid in sorted(table):
    for e in table[pid]:
        star = "  <<< INTEREST" if pid in INTEREST else ""
        print("%s %d (0x%X) handler=%s site=%s%s" % (
            e["kind"], pid, pid, e["handler"], e["site"], star))

# Decompile handlers for the damage-font ids of interest
print("=== decompiling interest handlers ===")
for pid in sorted(INTEREST):
    for e in table.get(pid, []):
        if not e["handler"]:
            continue
        fea = int(e["handler"], 16)
        nm = ida_name.get_name(fea) or "sub_%X" % fea
        try:
            cf = ida_hexrays.decompile(fea)
        except Exception as exc:
            print("FAIL %d %s %r" % (pid, nm, exc))
            continue
        if cf is None:
            print("NONE %d %s" % (pid, nm))
            continue
        text = str(cf)
        p = os.path.join(OUT_DIR, "df7_reg_%s%d_%s.c" % (e["kind"], pid, ("%X" % fea).lower()))
        open(p, "w", encoding="utf-8").write(text)
        print("OK %s%d handler %s lines=%d -> %s" % (e["kind"], pid, nm, text.count("\n"), p))

import json
json.dump({str(k): v for k, v in table.items()},
          open(os.path.join(OUT_DIR, "df7-registrar-table.json"), "w"), indent=2)
idc.qexit(0)

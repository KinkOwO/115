#!/usr/bin/env python3
"""Locate and decompile the skin-cargo packet handlers.

Anchor: analysis/dumps/opcodes.tsv gives each opcode a `table_slot_va`. For the
skin family the NOTI slots are 8 bytes apart (…4f8/…500/…508), i.e. an array of
qword handler pointers. We read the pointer at each slot, confirm it targets
code, decompile it, and also cross-check with the registrar-immediate heuristic
(functions that load the opcode id, per ida_noti_targeted.py). Nothing here
infers a byte layout from names: the decompiled handler text is the evidence.

Run:  idat -A -L<log> -S"ida_skin_recon.py" <work>/DFO.exe.i64
"""
import json
import os
import re
from datetime import datetime

import ida_bytes
import ida_funcs
import ida_hexrays
import ida_name
import ida_search
import ida_segment
import idaapi
import idautils
import idc

OUT_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "skin-noti")

# family, id, hex, name, string_va, table_slot_va  (from opcodes.tsv)
TARGETS = [
    ("noti", 1231, 0x04CF, "DAMAGE_FONT_SKIN_LIST", 0x14b010440, 0x14ef35b28),
    ("noti", 1545, 0x0609, "SKIN_CARGO_INFO",       0x14b019800, 0x14ef364f8),
    ("noti", 1546, 0x060A, "SELECT_SKIN_LIST",      0x14b019890, 0x14ef36500),
    ("noti", 1547, 0x060B, "RECENT_ADD_SKIN_LIST",  0x14b019910, 0x14ef36508),
    ("cmd",   857, 0x0359, "OPEN_AURA_SKIN_SLOT",   0x14b050fd0, 0x14ef3aa28),
    ("cmd",  1282, 0x0502, "SELECT_DAMAGE_FONT_SKIN",0x14b05a790,0x14ef3b770),
    ("cmd",  1565, 0x061D, "SELECT_SKIN",           0x14b060d90, 0x14ef3c048),
    ("cmd",  1592, 0x0638, "MAKE_SKIN",             0x14b0616e8, 0x14ef3c120),
]

NOTI_HELPER = 0x14599D5D0  # registrar used by ida_noti_targeted.py (build-specific)


def seg_of(ea):
    s = ida_segment.getseg(ea)
    return ida_segment.get_segm_name(s) if s else None


def is_code(ea):
    return ida_bytes.is_code(ida_bytes.get_flags(ea))


def read_str(ea, n=64):
    try:
        return idc.get_strlit_contents(ea, -1, idc.STRTYPE_C).decode("utf-8", "replace")
    except Exception:
        try:
            return ida_bytes.get_bytes(ea, n).split(b"\x00")[0].decode("utf-8", "replace")
        except Exception:
            return None


def funcs_with_immediate(value, cap=40):
    out = set()
    ea = 0
    flags = ida_search.SEARCH_DOWN | ida_search.SEARCH_NEXT
    for _ in range(cap):
        found = ida_search.find_imm(ea, flags, value)
        fea = found[0] if isinstance(found, tuple) else found
        if fea in (None, idaapi.BADADDR):
            break
        f = ida_funcs.get_func(fea)
        if f:
            out.add(f.start_ea)
        ea = fea + 1
    return sorted(out)


def decompile_to(ea, label):
    try:
        cf = ida_hexrays.decompile(ea)
    except Exception as exc:
        return {"ok": False, "error": repr(exc)}
    if cf is None:
        return {"ok": False, "error": "decompile returned None"}
    text = str(cf)
    path = os.path.join(OUT_DIR, label + ".c")
    with open(path, "w", encoding="utf-8") as fh:
        fh.write(text)
    return {"ok": True, "lines": text.count("\n"), "file": path}


def main():
    os.makedirs(OUT_DIR, exist_ok=True)
    report = {"generated": datetime.now().isoformat(timespec="seconds"),
              "noti_helper_present": is_code(NOTI_HELPER), "targets": []}
    for family, oid, hexid, name, string_va, slot_va in TARGETS:
        rec = {"family": family, "id": oid, "hex": hex(hexid), "name": name,
               "string_va": hex(string_va), "string": read_str(string_va),
               "slot_va": hex(slot_va), "slot_seg": seg_of(slot_va)}
        ptr = ida_bytes.get_qword(slot_va)
        rec["slot_qword"] = hex(ptr)
        rec["slot_qword_seg"] = seg_of(ptr)
        rec["slot_qword_is_code"] = is_code(ptr)
        rec["slot_qword_name"] = ida_name.get_ea_name(ptr) if ptr else None
        # decompile the handler the slot points at
        if ptr and is_code(ptr):
            f = ida_funcs.get_func(ptr)
            hea = f.start_ea if f else ptr
            rec["handler_ea"] = hex(hea)
            rec["handler_name"] = ida_funcs.get_func_name(hea)
            rec["handler_decompile"] = decompile_to(hea, "%s_%d_%s_handler_%x" % (family, oid, name.lower(), hea))
        # xrefs to the slot (who registers / dispatches through it)
        rec["slot_xrefs"] = []
        for x in list(idautils.DataRefsTo(slot_va)) + list(idautils.CodeRefsTo(slot_va, 0)):
            fn = ida_funcs.get_func(x)
            rec["slot_xrefs"].append({"ea": hex(x), "func": ida_funcs.get_func_name(fn.start_ea) if fn else None,
                                      "func_ea": hex(fn.start_ea) if fn else None})
        # cross-check: functions loading the opcode id as an immediate
        imm_funcs = funcs_with_immediate(hexid)
        rec["immediate_funcs"] = [{"ea": hex(a), "name": ida_funcs.get_func_name(a)} for a in imm_funcs[:20]]
        rec["immediate_func_count"] = len(imm_funcs)
        report["targets"].append(rec)
        print("[%s %d %-24s] slot=%s -> qword=%s code=%s name=%s imm_funcs=%d"
              % (family, oid, name, rec["slot_va"], rec["slot_qword"],
                 rec["slot_qword_is_code"], rec["slot_qword_name"], len(imm_funcs)))
    with open(os.path.join(OUT_DIR, "skin-recon.json"), "w", encoding="utf-8") as fh:
        json.dump(report, fh, indent=2, ensure_ascii=False)
    print("wrote skin-recon.json to %s" % OUT_DIR)
    idc.qexit(0)


if __name__ == "__main__":
    main()

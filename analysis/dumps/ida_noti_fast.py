#!/usr/bin/env python3
"""NOTI handler finder at disassembly level (fast, no Hex-Rays).

Hex-Rays decompiling 168 candidate functions took >25 minutes, so this pass
works on instructions instead:

  for every immediate hit of a target NOTI id,
    walk forward until the call to sub_14599D5D0 (the NOTI registry),
    then look back for the handler argument (r8, loaded with `lea`),

which is milliseconds per function.

Run inside IDA:

    idat -A -L<log> -S"analysis/dumps/ida_noti_fast.py" <work>.i64
"""

import json
import os
from datetime import datetime

import ida_funcs
import ida_search
import idaapi
import idautils
import idc

NOTI_HELPER = 0x14599D5D0
CMD_HELPER = 0x14599D450

TARGET_IDS = {
    0x05C2: "DUNGEON_TIMEOUT_TIME",
    0x08CC: "LEGION_BASIC_CLEAR_REWARD",
    0x08CD: "LEGION_ADDITIONAL_CLEAR_REWARD",
    0x08CE: "LEGION_ENTRY_CHARAC_INFO",
    0x0A08: "PREPARE_LEGION_ENTER_DUNGEON",
    0x0A61: "LEGION_PHASE_CLEAR_TICK",
    0x0B4F: "LEGION_INFO",
    0x0B50: "LEGION_OPERATION",
}

MAX_IMM_HITS = 60
FORWARD_STEPS = 12
BACKWARD_STEPS = 10
OUT_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "noti-fast")


def imm_hits(value):
    hits = []
    ea = 0
    flags = ida_search.SEARCH_DOWN | ida_search.SEARCH_NEXT
    while len(hits) < MAX_IMM_HITS:
        found = ida_search.find_imm(ea, flags, value)
        found_ea = found[0] if isinstance(found, tuple) else found
        if found_ea in (None, idaapi.BADADDR):
            break
        hits.append(found_ea)
        ea = found_ea + 1
    return hits


def walk_forward(ea, steps):
    out = []
    cursor = ea
    for _ in range(steps):
        cursor = idc.next_head(cursor, idc.get_segm_end(cursor))
        if cursor == idc.BADADDR:
            break
        out.append(cursor)
    return out


def walk_backward(ea, steps):
    out = []
    cursor = ea
    for _ in range(steps):
        cursor = idc.prev_head(cursor)
        if cursor == idc.BADADDR:
            break
        out.append(cursor)
    return out


def handler_argument(call_ea):
    """Find the function referenced near the call (the r8 argument)."""
    candidates = []
    for ea in [call_ea] + walk_backward(call_ea, BACKWARD_STEPS):
        text = idc.generate_disasm_line(ea, 0) or ""
        for target in idautils.CodeRefsFrom(ea, 0):
            func = ida_funcs.get_func(target)
            if func is not None and func.start_ea != NOTI_HELPER:
                candidates.append({"ea": hex(ea), "target": hex(target), "insn": text.strip()})
    return candidates


def main():
    if not os.path.isdir(OUT_DIR):
        os.makedirs(OUT_DIR)
    report = {"generated": datetime.now().isoformat(timespec="seconds"), "entries": []}
    for value, name in sorted(TARGET_IDS.items()):
        entries = []
        for hit in imm_hits(value):
            func = ida_funcs.get_func(hit)
            if func is None:
                continue
            for ea in walk_forward(hit, FORWARD_STEPS):
                text = idc.generate_disasm_line(ea, 0) or ""
                if "call" not in text:
                    continue
                for target in idautils.CodeRefsFrom(ea, 0):
                    if target not in (NOTI_HELPER, CMD_HELPER):
                        continue
                    entries.append({
                        "hit": hex(hit),
                        "call": hex(ea),
                        "helper": "noti" if target == NOTI_HELPER else "cmd",
                        "function": hex(func.start_ea),
                        "function_name": ida_funcs.get_func_name(func.start_ea),
                        "call_insn": text.strip(),
                        "handlers": handler_argument(ea),
                    })
        report["entries"].append({"id": value, "hex": hex(value), "name": name, "sites": entries})
        print("id %#06x %-32s sites=%d" % (value, name, len(entries)))
        for entry in entries[:6]:
            handlers = ", ".join(h["insn"] for h in entry["handlers"][:2])
            print("    %-14s %s | %s" % (entry["function_name"], entry["call_insn"], handlers[:80]))
    path = os.path.join(OUT_DIR, "noti-fast.json")
    with open(path, "w", encoding="utf-8") as handle:
        json.dump(report, handle, indent=2, ensure_ascii=False)
    print("wrote %s" % path)


if __name__ == "__main__":
    main()

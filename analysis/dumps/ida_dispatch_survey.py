#!/usr/bin/env python3
"""Third pass: how does the client map an opcode id to a handler?

Answers three questions in one run:

  A. string decoding - decompile sub_146E8C490 so the encoded enum names can be
     reproduced offline (validates the names in analysis/dumps/opcodes.tsv).
  B. decoded-name readers - who reads the globals the initializer fills
     (qword_14EF3CF38 ...)? No reader means the names are logging only.
  C. opcode dispatch - locate immediate comparisons against the legion /
     apocalypse opcode ids (0x7FB, 0x932, 0x933, 0xB4F, ...) and report the
     containing functions, which is where the id -> handler mapping lives.

Run inside IDA:

    idat -A -L<log> -S"analysis/dumps/ida_dispatch_survey.py" <work>.i64
"""

import json
import os
from datetime import datetime

import ida_funcs
import ida_hexrays
import ida_search
import idaapi
import idautils
import idc

DECODERS = [
    ("string_decoder", 0x146E8C490),
    ("string_decoder_entry", 0x146E8C7D0),
]

# Decoded-name globals filled by the initializer (from the second pass).
NAME_GLOBALS = [
    (0x14EF3CF38, "cmd LEGION_START"),
    (0x14EF3CF40, "cmd LEGION_FAIL"),
    (0x14EF3CF48, "cmd LEGION_ENTER_DUNGEON"),
    (0x14EF3CF50, "cmd LEGION_REWARD_END"),
    (0x14EF3D6F0, "cmd VENUS_OPERATION_SELECT"),
    (0x14EF3D708, "cmd VENUS_END_AT_PHASE4"),
    (0x14EF3D8F0, "cmd LEGION_OPERATION_SELECT"),
    (0x14EF3D8F8, "cmd APOCALYPSE_ROLE_SELECT"),
    (0x14EF3DB20, "cmd LEGION_ACHIEVEMENT_ACTION"),
    (0x14EF362C0, "noti DUNGEON_TIMEOUT_TIME"),
    (0x14EF37B10, "noti LEGION_BASIC_CLEAR_REWARD"),
    (0x14EF37B18, "noti LEGION_ADDITIONAL_CLEAR_REWARD"),
    (0x14EF37B20, "noti LEGION_ENTRY_CHARAC_INFO"),
    (0x14EF384F0, "noti PREPARE_LEGION_ENTER_DUNGEON"),
    (0x14EF387B8, "noti LEGION_PHASE_CLEAR_TICK"),
    (0x14EF38F28, "noti LEGION_INFO"),
    (0x14EF38F30, "noti LEGION_OPERATION"),
]

# opcode ids to hunt for as immediates.
OPCODE_IDS = [
    ("cmd", 0x07FB, "LEGION_START"),
    ("cmd", 0x07FC, "LEGION_FAIL"),
    ("cmd", 0x07FD, "LEGION_ENTER_DUNGEON"),
    ("cmd", 0x07FE, "LEGION_REWARD_END"),
    ("cmd", 0x08F2, "VENUS_OPERATION_SELECT"),
    ("cmd", 0x08F5, "VENUS_END_AT_PHASE4"),
    ("cmd", 0x0932, "LEGION_OPERATION_SELECT"),
    ("cmd", 0x0933, "APOCALYPSE_ROLE_SELECT"),
    ("cmd", 0x0978, "LEGION_ACHIEVEMENT_ACTION"),
    ("noti", 0x05C2, "DUNGEON_TIMEOUT_TIME"),
    ("noti", 0x08CC, "LEGION_BASIC_CLEAR_REWARD"),
    ("noti", 0x08CD, "LEGION_ADDITIONAL_CLEAR_REWARD"),
    ("noti", 0x08CE, "LEGION_ENTRY_CHARAC_INFO"),
    ("noti", 0x0A08, "PREPARE_LEGION_ENTER_DUNGEON"),
    ("noti", 0x0A61, "LEGION_PHASE_CLEAR_TICK"),
    ("noti", 0x0B4F, "LEGION_INFO"),
    ("noti", 0x0B50, "LEGION_OPERATION"),
]

MAX_IMM_HITS = 12
MAX_DECOMPILE_LINES = 900
OUT_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "dispatch-survey")


def decompile_text(ea, limit=MAX_DECOMPILE_LINES):
    try:
        cfunc = ida_hexrays.decompile(ea)
    except Exception as exc:
        return "hexrays failed: %s" % exc, 0
    if cfunc is None:
        return "", 0
    lines = str(cfunc).splitlines()
    total = len(lines)
    if total > limit:
        lines = lines[:limit] + ["... (%d more lines)" % (total - limit)]
    return "\n".join(lines), total


def listing(ea, before=0x20, after=0x10):
    func = ida_funcs.get_func(ea)
    if func is None:
        return []
    out = []
    for item in idautils.FuncItems(func.start_ea):
        if item < ea - before or item > ea + after:
            continue
        out.append("%#x  %s" % (item, idc.generate_disasm_line(item, 0)))
    return out


def readers_of(global_va):
    """Code references to a decoded-name global, excluding the initializer."""
    out = []
    for xref in idautils.XrefsTo(global_va, 0):
        func = ida_funcs.get_func(xref.frm)
        out.append({
            "from": hex(xref.frm),
            "type": int(xref.type),
            "func": hex(func.start_ea) if func else None,
            "func_name": ida_funcs.get_func_name(func.start_ea) if func else None,
            "listing": listing(xref.frm),
        })
    return out


def find_immediate_hits(value):
    """Instructions whose immediate operand equals value."""
    hits = []
    ea = 0
    flags = ida_search.SEARCH_DOWN | ida_search.SEARCH_NEXT
    while len(hits) < MAX_IMM_HITS:
        found = ida_search.find_imm(ea, flags, value)
        found_ea = found[0] if isinstance(found, tuple) else found
        if found_ea == idaapi.BADADDR or found_ea is None:
            break
        func = ida_funcs.get_func(found_ea)
        hits.append({
            "ea": hex(found_ea),
            "insn": idc.generate_disasm_line(found_ea, 0),
            "func": hex(func.start_ea) if func else None,
            "func_name": ida_funcs.get_func_name(func.start_ea) if func else None,
        })
        ea = found_ea + 1
    return hits


def main():
    if not os.path.isdir(OUT_DIR):
        os.makedirs(OUT_DIR)
    report = {"generated": datetime.now().isoformat(timespec="seconds"), "decoders": [], "name_readers": [], "dispatch": []}

    for label, ea in DECODERS:
        text, total = decompile_text(ea)
        path = os.path.join(OUT_DIR, "%s_%x.c" % (label, ea))
        with open(path, "w", encoding="utf-8") as handle:
            handle.write(text + "\n")
        report["decoders"].append({"label": label, "ea": hex(ea), "lines": total, "file": path})
        print("[decoder] %s %#x lines=%d" % (label, ea, total))

    for va, label in NAME_GLOBALS:
        readers = readers_of(va)
        entry = {"global": hex(va), "label": label, "readers": readers}
        report["name_readers"].append(entry)
        print("[reader ] %-32s refs=%d %s" % (label, len(readers),
                                              sorted({r["func_name"] for r in readers})[:3]))

    for family, value, name in OPCODE_IDS:
        hits = find_immediate_hits(value)
        report["dispatch"].append({"family": family, "id": hex(value), "name": name, "hits": hits})
        print("[imm    ] %s %#06x %-32s hits=%d %s" % (family, value, name, len(hits),
                                                       sorted({h["func_name"] for h in hits})[:3]))

    path = os.path.join(OUT_DIR, "dispatch-survey.json")
    with open(path, "w", encoding="utf-8") as handle:
        json.dump(report, handle, indent=2, ensure_ascii=False)
    print("wrote %s" % path)


if __name__ == "__main__":
    main()

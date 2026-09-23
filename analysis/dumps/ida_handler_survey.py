#!/usr/bin/env python3
"""Second pass: pull the legion/apocalypse handlers out of the registration site.

The first pass (ida_legion_survey.py) showed every legion/apocalypse opcode
string is referenced exactly once, all inside sub_140069BB0 - the registration
function. This pass decompiles that function and prints, for every opcode
reference, the surrounding instructions so the handler address (usually a
`lea reg, sub_XXX` right next to the enum string) can be read off.

Run inside IDA (headless or GUI):

    idat -A -L<log> -S"analysis/dumps/ida_handler_survey.py" <work>.i64
"""

import json
import os
from datetime import datetime

import ida_bytes
import ida_funcs
import ida_hexrays
import ida_name
import idautils
import idc

REGISTRATION_FUNC = 0x140069BB0

# (family, opcode, enum name) - same set as the first pass.
OPCODES = [
    ("cmd", 2043, "LEGION_START"),
    ("cmd", 2044, "LEGION_FAIL"),
    ("cmd", 2045, "LEGION_ENTER_DUNGEON"),
    ("cmd", 2046, "LEGION_REWARD_END"),
    ("cmd", 2290, "VENUS_OPERATION_SELECT"),
    ("cmd", 2293, "VENUS_END_AT_PHASE4"),
    ("cmd", 2354, "LEGION_OPERATION_SELECT"),
    ("cmd", 2355, "APOCALYPSE_ROLE_SELECT"),
    ("cmd", 2424, "LEGION_ACHIEVEMENT_ACTION"),
    ("noti", 1474, "DUNGEON_TIMEOUT_TIME"),
    ("noti", 2252, "LEGION_BASIC_CLEAR_REWARD"),
    ("noti", 2253, "LEGION_ADDITIONAL_CLEAR_REWARD"),
    ("noti", 2254, "LEGION_ENTRY_CHARAC_INFO"),
    ("noti", 2568, "PREPARE_LEGION_ENTER_DUNGEON"),
    ("noti", 2657, "LEGION_PHASE_CLEAR_TICK"),
    ("noti", 2895, "LEGION_INFO"),
    ("noti", 2896, "LEGION_OPERATION"),
]

# (slot VA, name) - the handler table cells, from analysis/dumps/opcodes.tsv.
SLOTS = [
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

WINDOW_BEFORE = 0x60
WINDOW_AFTER = 0x20

OUT_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "legion-survey")


def disasm_window(ea, before=WINDOW_BEFORE, after=WINDOW_AFTER):
    """Linear listing around ea, tagged with function references and name hints."""
    func = ida_funcs.get_func(ea)
    start = max(ea - before, func.start_ea if func else ea - before)
    lines = []
    for item in idautils.FuncItems(func.start_ea) if func else []:
        if item < start or item > ea + after:
            continue
        text = idc.generate_disasm_line(item, 0)
        note = ""
        for target in idautils.CodeRefsFrom(item, 0):
            target_func = ida_funcs.get_func(target)
            if target_func is not None:
                note = "  ; -> %#x %s" % (target_func.start_ea, ida_funcs.get_func_name(target_func.start_ea))
        marker = "  <<< opcode registration" if item == ea else ""
        lines.append("%#x  %-40s%s%s" % (item, text, note, marker))
    return lines


def handler_candidates(ea, before=WINDOW_BEFORE, after=WINDOW_AFTER):
    """Address-looking immediates in the window, i.e. probable handler pointers."""
    func = ida_funcs.get_func(ea)
    if func is None:
        return []
    out = []
    for item in idautils.FuncItems(func.start_ea):
        if item < ea - before or item > ea + after:
            continue
        for target in idautils.CodeRefsFrom(item, 0):
            target_func = ida_funcs.get_func(target)
            if target_func is not None and target_func.start_ea != func.start_ea:
                out.append({
                    "from": hex(item),
                    "callee": hex(target_func.start_ea),
                    "callee_name": ida_funcs.get_func_name(target_func.start_ea),
                })
    return out


def main():
    if not os.path.isdir(OUT_DIR):
        os.makedirs(OUT_DIR)
    result = {
        "generated": datetime.now().isoformat(timespec="seconds"),
        "registration_func": hex(REGISTRATION_FUNC),
        "registration_name": ida_funcs.get_func_name(REGISTRATION_FUNC),
        "decompile": None,
        "ops": [],
        "slots": [],
    }

    # 1. Full pseudocode of the registration function.
    try:
        cfunc = ida_hexrays.decompile(REGISTRATION_FUNC)
        text = str(cfunc) if cfunc else ""
    except Exception as exc:
        text = "hexrays failed: %s" % exc
    if text:
        lines = text.splitlines()
        if len(lines) > 2500:
            text = "\n".join(lines[:2500]) + "\n... (%d more lines)" % (len(lines) - 2500)
        with open(os.path.join(OUT_DIR, "registration_140069bb0.c"), "w", encoding="utf-8") as handle:
            handle.write(text)
        result["decompile"] = "registration_140069bb0.c (%d lines)" % len(lines)

    # 2. Per opcode: what the registration site looks like and which functions it touches.
    for family, opcode, name in OPCODES:
        slot = None
        for va, label in SLOTS:
            if label == "%s %s" % (family, name):
                slot = va
        result["ops"].append({"family": family, "opcode": opcode, "name": name, "slot": hex(slot) if slot else None})

    # xrefs of the enum strings come from the manifest of the first pass.
    manifest = os.path.join(OUT_DIR, "survey.json")
    if os.path.exists(manifest):
        with open(manifest, encoding="utf-8") as handle:
            first = json.load(handle)
        by_name = {r["name"]: r for r in first.get("opcodes", [])}
    else:
        by_name = {}
    for record in result["ops"]:
        entry = by_name.get(record["name"], {})
        refs = entry.get("string_xrefs") or []
        sites = []
        for ref in refs:
            ea = int(ref["ea"], 16)
            sites.append({
                "xref": ref["ea"],
                "func": ref.get("func"),
                "listing": disasm_window(ea),
                "candidates": handler_candidates(ea),
            })
        record["sites"] = sites

    # 3. Per slot: who writes it (the store instruction reveals the handler).
    for va, label in SLOTS:
        writers = []
        for xref in idautils.DataRefsTo(va):
            func = ida_funcs.get_func(xref)
            writers.append({
                "ea": hex(xref),
                "func": hex(func.start_ea) if func else None,
                "func_name": ida_funcs.get_func_name(func.start_ea) if func else None,
                "listing": disasm_window(xref, before=0x30, after=0x10),
            })
        # Also report xrefs to the nearest 8 bytes of the table (some stores use an indexed base).
        result["slots"].append({"slot": hex(va), "label": label, "writers": writers})

    path = os.path.join(OUT_DIR, "handler-survey.json")
    with open(path, "w", encoding="utf-8") as handle:
        json.dump(result, handle, indent=2, ensure_ascii=False)
    print("registration: %s" % result["registration_name"])
    print("decompile: %s" % result["decompile"])
    slots_with_writers = sum(1 for s in result["slots"] if s["writers"])
    print("slots with code writers: %d/%d" % (slots_with_writers, len(result["slots"])))
    for record in result["ops"]:
        for site in record.get("sites", []):
            print("[%s %s] xref %s candidates=%d" % (
                record["family"], record["name"], site["xref"], len(site["candidates"])))
    print("wrote %s" % path)


if __name__ == "__main__":
    main()

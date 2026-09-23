#!/usr/bin/env python3
"""Fourth pass: extract (opcode id -> handler) registrations and (opcode id -> sender) sites.

Leads from pass three:
  * sub_14599D450(registry, id, function, ...) registers a handler for a packet id
    (seen in sub_14069E490 registering 2355).
  * sub_146D746E0(writer, id) stamps the opcode on an outgoing packet
    (seen in sub_14069E3B0, the client's APOCALYPSE_ROLE_SELECT sender).

For every call site of those two helpers this pass scans the preceding window
for an immediate operand equal to one of the legion/apocalypse ids and reports
the containing function plus any function referenced nearby (the handler).

Run inside IDA:

    idat -A -L<log> -S"analysis/dumps/ida_registry_survey.py" <work>.i64
"""

import json
import os
from datetime import datetime

import ida_funcs
import idautils
import idc

REGISTER_HELPER = 0x14599D450   # register(id, handler)
SEND_HELPER = 0x146D746E0       # outgoing packet: set opcode

IDS = [
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

WINDOW = 0x40
OUT_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "registry-survey")


def window_scan(call_ea):
    """Immediates and referenced functions inside the window before a call."""
    func = ida_funcs.get_func(call_ea)
    start = max(call_ea - WINDOW, func.start_ea if func else call_ea - WINDOW)
    immediates = {}
    functions = []
    listing = []
    for item in idautils.FuncItems(func.start_ea) if func else []:
        if item < start or item > call_ea:
            continue
        listing.append("%#x  %s" % (item, idc.generate_disasm_line(item, 0)))
        for index in range(3):
            value = idc.get_operand_value(item, index)
            if value not in (0, idc.BADADDR):
                immediates.setdefault(value, item)
        for target in idautils.CodeRefsFrom(item, 0):
            target_func = ida_funcs.get_func(target)
            if target_func is not None and target_func.start_ea != (func.start_ea if func else 0):
                functions.append({
                    "ea": hex(target_func.start_ea),
                    "name": ida_funcs.get_func_name(target_func.start_ea),
                    "from": hex(item),
                })
    return immediates, functions, listing


def survey_helper(helper_ea, label, report):
    func = ida_funcs.get_func(helper_ea)
    entries = []
    for xref in idautils.CodeRefsTo(helper_ea, 0):
        # CodeRefsTo yields plain addresses; XrefsTo yields objects. Accept both.
        call_ea = xref if isinstance(xref, int) else xref.frm
        immediates, functions, listing = window_scan(call_ea)
        matched = []
        for family, value, name in IDS:
            if value in immediates:
                matched.append({"family": family, "id": hex(value), "name": name, "imm_at": hex(immediates[value])})
        if not matched:
            continue
        caller = ida_funcs.get_func(call_ea)
        entries.append({
            "call": hex(call_ea),
            "caller": hex(caller.start_ea) if caller else None,
            "caller_name": ida_funcs.get_func_name(caller.start_ea) if caller else None,
            "matched_ids": matched,
            "referenced_functions": functions,
            "listing": listing,
        })
    report[label] = entries
    print("[%s] call sites with a matching id: %d" % (label, len(entries)))
    for entry in entries[:20]:
        ids = ", ".join("%s(%s)" % (m["name"], m["id"]) for m in entry["matched_ids"])
        refs = ", ".join(sorted({f["name"] for f in entry["referenced_functions"]})[:3])
        print("   %-14s %s -> refs %s" % (entry["caller_name"], ids, refs))


def main():
    if not os.path.isdir(OUT_DIR):
        os.makedirs(OUT_DIR)
    report = {"generated": datetime.now().isoformat(timespec="seconds"), "register": {}, "send": {}}
    survey_helper(REGISTER_HELPER, "register", report)
    survey_helper(SEND_HELPER, "send", report)
    path = os.path.join(OUT_DIR, "registry-survey.json")
    with open(path, "w", encoding="utf-8") as handle:
        json.dump(report, handle, indent=2, ensure_ascii=False)
    print("wrote %s" % path)


if __name__ == "__main__":
    main()

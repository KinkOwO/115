#!/usr/bin/env python3
"""Seventh pass: NOTI handlers, targeted by immediate search.

Decompiling every registrar (608 functions) is too slow. This pass instead
finds the instructions that load one of the target NOTI ids as an immediate,
takes the containing functions only, and extracts the handler from the
sub_14599D5D0 call inside them.

Run inside IDA:

    idat -A -L<log> -S"analysis/dumps/ida_noti_targeted.py" <work>.i64
"""

import json
import os
import re
from datetime import datetime

import ida_funcs
import ida_hexrays
import ida_search
import idaapi
import idautils

NOTI_HELPER = 0x14599D5D0

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

MAX_HITS_PER_ID = 60
MAX_DECOMPILE = 120
OUT_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "noti-targeted")
CALL_RE = re.compile(r"sub_14599D5D0\(([^;]{0,220})\);")


def functions_with_immediate(value):
    funcs = set()
    ea = 0
    flags = ida_search.SEARCH_DOWN | ida_search.SEARCH_NEXT
    count = 0
    while count < MAX_HITS_PER_ID:
        found = ida_search.find_imm(ea, flags, value)
        found_ea = found[0] if isinstance(found, tuple) else found
        if found_ea in (None, idaapi.BADADDR):
            break
        func = ida_funcs.get_func(found_ea)
        if func is not None:
            funcs.add(func.start_ea)
        ea = found_ea + 1
        count += 1
    return funcs


def main():
    if not os.path.isdir(OUT_DIR):
        os.makedirs(OUT_DIR)
    candidates = {}
    for value, name in TARGET_IDS.items():
        funcs = functions_with_immediate(value)
        candidates[value] = funcs
        print("id %#06x %-32s candidate functions: %d" % (value, name, len(funcs)))

    todo = sorted({f for funcs in candidates.values() for f in funcs})
    print("distinct functions to decompile: %d" % len(todo))
    pairs = []
    dumped = 0
    for index, func_ea in enumerate(todo):
        text = None
        cache = os.path.join(OUT_DIR, "func_%x.c" % func_ea)
        if os.path.exists(cache):
            text = open(cache, encoding="utf-8").read()
        else:
            try:
                cfunc = ida_hexrays.decompile(func_ea)
            except Exception:
                cfunc = None
            if cfunc is None:
                continue
            lines = str(cfunc).splitlines()
            if len(lines) > MAX_DECOMPILE:
                lines = lines[:MAX_DECOMPILE]
            text = "\n".join(lines)
            with open(cache, "w", encoding="utf-8") as handle:
                handle.write(text + "\n")
            dumped += 1
        if "sub_14599D5D0" not in text:
            continue
        for match in CALL_RE.finditer(text):
            args = match.group(1)
            id_match = re.search(r",\s*(0x[0-9A-Fa-f]+)u?\s*,", args)
            handler_match = re.search(r",\s*\(?(?:__int64)\)?\s*(sub_[0-9A-Fa-f]+)", args)
            if not id_match or not handler_match:
                continue
            value = int(id_match.group(1), 16)
            if value not in TARGET_IDS:
                continue
            pairs.append({
                "id": value,
                "hex": hex(value),
                "name": TARGET_IDS[value],
                "handler": handler_match.group(1),
                "registrar": ida_funcs.get_func_name(func_ea),
                "args": args.strip()[:160],
            })
        if index % 25 == 0:
            print("  progress %d/%d dumped=%d pairs=%d" % (index, len(todo), dumped, len(pairs)))
    print("dumped %d functions, target pairs %d" % (dumped, len(pairs)))
    seen = set()
    for record in pairs:
        key = (record["id"], record["handler"])
        if key in seen:
            continue
        seen.add(key)
        print("  %-6s %-32s -> %s (%s)" % (record["hex"], record["name"], record["handler"], record["registrar"]))
    with open(os.path.join(OUT_DIR, "noti-handlers.json"), "w", encoding="utf-8") as handle:
        json.dump({"generated": datetime.now().isoformat(timespec="seconds"), "pairs": pairs}, handle,
                  indent=2, ensure_ascii=False)
    print("wrote noti-handlers.json")


if __name__ == "__main__":
    main()

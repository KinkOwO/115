#!/usr/bin/env python3
"""Sixth pass: NOTI-side handler map.

sub_14599D5D0 is the sibling of sub_14599D450 (same (registry, id, handler, flags)
shape, lazily creates its own 136 byte registry). This pass decompiles every
caller that registers one of the legion/apocalypse NOTI ids and extracts the
(id, handler) pairs, giving the S2C side of the family.

Run inside IDA:

    idat -A -L<log> -S"analysis/dumps/ida_noti_registry.py" <work>.i64
"""

import json
import os
import re
from datetime import datetime

import ida_funcs
import ida_hexrays
import idautils

NOTI_HELPER = 0x14599D5D0
CMD_HELPER = 0x14599D450

# NOTI ids we still need handlers for.
TARGET_IDS = {
    0x05C2: "DUNGEON_TIMEOUT_TIME",
    0x08CC: "LEGION_BASIC_CLEAR_REWARD",
    0x08CD: "LEGION_ADDITIONAL_CLEAR_REWARD",
    0x08CE: "LEGION_ENTRY_CHARAC_INFO",
    0x0A08: "PREPARE_LEGION_ENTER_DUNGEON",
    0x0A61: "LEGION_PHASE_CLEAR_TICK",
    0x0B4F: "LEGION_INFO",
    0x0B50: "LEGION_OPERATION",
    0x0766: "TOWER_OF_GRAVE_RESULT",
}

MAX_LINES = 260
OUT_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "noti-registry")
CALL_RE = re.compile(r"sub_14599D5D0\(([^;]{0,220})\);")


def decompile(ea):
    try:
        cfunc = ida_hexrays.decompile(ea)
    except Exception as exc:
        return "hexrays failed: %s" % exc
    if cfunc is None:
        return ""
    lines = str(cfunc).splitlines()
    if len(lines) > MAX_LINES:
        lines = lines[:MAX_LINES] + ["... (%d more lines)" % (len(lines) - MAX_LINES)]
    return "\n".join(lines)


def main():
    if not os.path.isdir(OUT_DIR):
        os.makedirs(OUT_DIR)
    callers = {}
    for ref in idautils.CodeRefsTo(NOTI_HELPER, 0):
        ea = ref if isinstance(ref, int) else ref.frm
        func = ida_funcs.get_func(ea)
        if func is not None:
            callers.setdefault(func.start_ea, []).append(ea)
    print("call sites of the NOTI helper: %d in %d functions" % (
        sum(len(v) for v in callers.values()), len(callers)))

    pairs = []
    dumped = 0
    for func_ea in sorted(callers):
        text = decompile(func_ea)
        if not text:
            continue
        found = []
        for match in CALL_RE.finditer(text):
            args = match.group(1)
            id_match = re.search(r",\s*(0x[0-9A-Fa-f]+)u?\s*,", args)
            handler_match = re.search(r",\s*\(?(?:__int64)\)?\s*(sub_[0-9A-Fa-f]+)", args)
            if not id_match or not handler_match:
                continue
            value = int(id_match.group(1), 16)
            label = TARGET_IDS.get(value)
            record = {
                "id": value,
                "hex": hex(value),
                "name": label or "",
                "handler": handler_match.group(1),
                "registrar": ida_funcs.get_func_name(func_ea),
                "args": args.strip()[:160],
            }
            found.append(record)
            if label:
                pairs.append(record)
        if found:
            name = "noti_register_%x.c" % func_ea
            with open(os.path.join(OUT_DIR, name), "w", encoding="utf-8") as handle:
                handle.write(text + "\n")
            dumped += 1
    print("registrar functions dumped: %d, target-id pairs: %d" % (dumped, len(pairs)))
    seen = set()
    for record in pairs:
        key = (record["id"], record["handler"])
        if key in seen:
            continue
        seen.add(key)
        print("  %-6s %-32s -> %s   (%s)" % (record["hex"], record["name"], record["handler"], record["registrar"]))
    all_pairs = pairs
    path = os.path.join(OUT_DIR, "noti-handlers.json")
    with open(path, "w", encoding="utf-8") as handle:
        json.dump({"generated": datetime.now().isoformat(timespec="seconds"), "pairs": all_pairs}, handle,
                  indent=2, ensure_ascii=False)
    print("wrote %s" % path)


if __name__ == "__main__":
    main()

#!/usr/bin/env python3
"""NOTI handler finder over the registry's call sites (fast).

find_imm per id is quadratic on a 150 MB .text and timed out. The registry has
"only" ~1200 call sites, so this pass walks those instead: for every call to
sub_14599D5D0 / sub_14599D450 it looks at a small instruction window for one of
the target ids plus the handler argument.

Run inside IDA:

    idat -A -L<log> -S"analysis/dumps/ida_noti_callsites.py" <work>.i64
"""

import json
import os
from datetime import datetime

import ida_funcs
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
    0x0766: "TOWER_OF_GRAVE_RESULT",
}

WINDOW = 14
OUT_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "noti-callsites")


def window(ea):
    out = []
    cursor = ea
    for _ in range(WINDOW):
        cursor = idc.prev_head(cursor)
        if cursor in (idc.BADADDR, None):
            break
        out.append(cursor)
    out.reverse()
    out.append(ea)
    return out


def main():
    if not os.path.isdir(OUT_DIR):
        os.makedirs(OUT_DIR)
    report = {"generated": datetime.now().isoformat(timespec="seconds"), "pairs": [], "all_sites": 0}
    for helper, kind in ((NOTI_HELPER, "noti"), (CMD_HELPER, "cmd")):
        sites = list(idautils.CodeRefsTo(helper, 0))
        report["all_sites"] += len(sites)
        print("[%s] call sites: %d" % (kind, len(sites)))
        for index, ref in enumerate(sites):
            call_ea = ref if isinstance(ref, int) else ref.frm
            ids = []
            handlers = []
            listing = []
            for ea in window(call_ea):
                text = (idc.generate_disasm_line(ea, 0) or "").strip()
                listing.append("%#x  %s" % (ea, text))
                for operand in range(3):
                    value = idc.get_operand_value(ea, operand)
                    if value in TARGET_IDS:
                        ids.append((value, ea))
                for target in idautils.CodeRefsFrom(ea, 0):
                    func = ida_funcs.get_func(target)
                    if func is not None and func.start_ea not in (NOTI_HELPER, CMD_HELPER):
                        handlers.append({"ea": hex(ea), "target": hex(target),
                                         "name": ida_funcs.get_func_name(target)})
            if not ids:
                continue
            func = ida_funcs.get_func(call_ea)
            for value, imm_ea in ids:
                report["pairs"].append({
                    "kind": kind,
                    "id": value,
                    "hex": hex(value),
                    "name": TARGET_IDS[value],
                    "call": hex(call_ea),
                    "id_load": hex(imm_ea),
                    "registrar": hex(func.start_ea) if func else None,
                    "registrar_name": ida_funcs.get_func_name(func.start_ea) if func else None,
                    "handlers": handlers,
                    "listing": listing,
                })
        print("  scanned %d sites" % len(sites))
    seen = set()
    for pair in report["pairs"]:
        key = (pair["kind"], pair["id"], pair["registrar"])
        if key in seen:
            continue
        seen.add(key)
        handler = ", ".join(h["name"] for h in pair["handlers"][:3])
        print("  %-4s %-6s %-32s registrar=%-14s handlers=%s" % (
            pair["kind"], pair["hex"], pair["name"], pair["registrar_name"], handler[:60]))
    path = os.path.join(OUT_DIR, "noti-callsites.json")
    with open(path, "w", encoding="utf-8") as handle:
        json.dump(report, handle, indent=2, ensure_ascii=False)
    print("pairs=%d wrote %s" % (len(report["pairs"]), path))


if __name__ == "__main__":
    main()

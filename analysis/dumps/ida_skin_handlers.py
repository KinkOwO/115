#!/usr/bin/env python3
"""Extract and decompile the skin-cargo packet handlers.

Recon showed the dispatch-table slots are runtime-filled (static 0xFF..FF) by two
builders: sub_140075000 (NOTI) and sub_140069BB0 (CMD). So we use the proven
registrar heuristic instead: NOTI/CMD handlers are registered through
sub_14599D5D0(<...>, <id>, <...>, <handler>). We find functions that load each
skin opcode id as an immediate, decompile them, regex out the sub_14599D5D0 call
to map id -> handler, then decompile each handler in full so the packet reader
call sequence (the byte layout) is visible as evidence.

Run:  idat -A -L<log> -S"ida_skin_handlers.py" <work>/DFO.exe.i64
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
import idc

OUT_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "skin-noti")
NOTI_HELPER = 0x14599D5D0
BUILDERS = {"noti_builder": 0x140075000, "cmd_builder": 0x140069BB0}

TARGET_IDS = {
    0x04CF: "DAMAGE_FONT_SKIN_LIST",
    0x0609: "SKIN_CARGO_INFO",
    0x060A: "SELECT_SKIN_LIST",
    0x060B: "RECENT_ADD_SKIN_LIST",
    0x0359: "OPEN_AURA_SKIN_SLOT",
    0x0502: "SELECT_DAMAGE_FONT_SKIN",
    0x061D: "SELECT_SKIN",
    0x0638: "MAKE_SKIN",
}
# functions that reference dozens of ids (generic tables/switches) -> skip as handlers
NOISE = {0x141192BC0, 0x1412327C0}

MAX_HITS = 60
CALL_RE = re.compile(r"sub_14599D5D0\(([^;]{0,260})\);")


def funcs_with_immediate(value):
    out = set()
    ea = 0
    flags = ida_search.SEARCH_DOWN | ida_search.SEARCH_NEXT
    for _ in range(MAX_HITS):
        found = ida_search.find_imm(ea, flags, value)
        fea = found[0] if isinstance(found, tuple) else found
        if fea in (None, idaapi.BADADDR):
            break
        f = ida_funcs.get_func(fea)
        if f:
            out.add(f.start_ea)
        ea = fea + 1
    return sorted(out)


def decompile_text(ea):
    try:
        cf = ida_hexrays.decompile(ea)
    except Exception:
        return None
    return None if cf is None else str(cf)


def dump(ea, label):
    text = decompile_text(ea)
    if text is None:
        return None
    path = os.path.join(OUT_DIR, label + ".c")
    with open(path, "w", encoding="utf-8") as fh:
        fh.write(text)
    return path


def main():
    os.makedirs(OUT_DIR, exist_ok=True)
    report = {"generated": datetime.now().isoformat(timespec="seconds"),
              "builders": {}, "ids": {}}

    for label, ea in BUILDERS.items():
        p = dump(ea, "builder_%s_%x" % (label, ea))
        report["builders"][label] = {"ea": hex(ea), "file": p}
        print("builder %-12s %x -> %s" % (label, ea, p))

    handlers = {}
    for value, name in TARGET_IDS.items():
        funcs = [f for f in funcs_with_immediate(value) if f not in NOISE]
        rec = {"name": name, "hex": hex(value), "registrar_funcs": [hex(f) for f in funcs], "handlers": []}
        for fea in funcs:
            text = decompile_text(fea)
            if not text or "sub_14599D5D0" not in text:
                continue
            for m in CALL_RE.finditer(text):
                args = m.group(1)
                idm = re.search(r",\s*(0x[0-9A-Fa-f]+)u?\s*,", args)
                hm = re.search(r",\s*\(?(?:__int64)?\)?\s*(sub_[0-9A-Fa-f]+)", args)
                if not idm or not hm:
                    continue
                if int(idm.group(1), 16) != value:
                    continue
                hname = hm.group(1)
                hea = int(hname[4:], 16)
                rec["handlers"].append({"handler": hname, "handler_ea": hex(hea),
                                        "registrar": ida_funcs.get_func_name(fea), "args": args.strip()[:200]})
                handlers.setdefault(hea, set()).add(name)
        report["ids"][hex(value)] = rec
        print("id %#06x %-24s registrar_funcs=%d handlers=%d" % (value, name, len(funcs), len(rec["handlers"])))

    dumped = {}
    for hea, names in sorted(handlers.items()):
        label = "handler_%x_%s" % (hea, "_".join(sorted(names))[:40])
        p = dump(hea, label)
        dumped[hex(hea)] = {"names": sorted(names), "file": p}
        print("  handler %x (%s) -> %s" % (hea, ",".join(sorted(names)), p))
    report["handlers"] = dumped

    with open(os.path.join(OUT_DIR, "skin-handlers.json"), "w", encoding="utf-8") as fh:
        json.dump(report, fh, indent=2, ensure_ascii=False)
    print("wrote skin-handlers.json")
    idc.qexit(0)


if __name__ == "__main__":
    main()

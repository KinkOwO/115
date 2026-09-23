#!/usr/bin/env python3
"""Fifth pass: expand the registry survey into a readable id -> function table.

Reads registry-survey/registry-survey.json (pass four) and, for every hit,
decompiles the containing function plus the functions it references near the
call (the handler for registrations). Writes one .c per function and a summary
table with plain columns:

  family  id  name  direction  function  handler/referenced

Run inside IDA:

    idat -A -L<log> -S"analysis/dumps/ida_registry_expand.py" <work>.i64
"""

import json
import os
from datetime import datetime

import ida_hexrays
import ida_funcs

MAX_LINES = 400
BASE = os.path.dirname(os.path.abspath(__file__))
SURVEY = os.path.join(BASE, "registry-survey", "registry-survey.json")
OUT_DIR = os.path.join(BASE, "registry-expand")


def decompile(ea):
    try:
        cfunc = ida_hexrays.decompile(ea)
    except Exception as exc:
        return "hexrays failed: %s" % exc, 0
    if cfunc is None:
        return "", 0
    lines = str(cfunc).splitlines()
    total = len(lines)
    if total > MAX_LINES:
        lines = lines[:MAX_LINES] + ["... (%d more lines)" % (total - MAX_LINES)]
    return "\n".join(lines), total


def dump(label, ea, table):
    text, total = decompile(ea)
    if not text:
        return total
    name = "%s_%x.c" % (label, ea)
    with open(os.path.join(OUT_DIR, name), "w", encoding="utf-8") as handle:
        handle.write(text + "\n")
    table.append({"function": hex(ea), "name": ida_funcs.get_func_name(ea), "lines": total, "file": name})
    return total


def main():
    if not os.path.isdir(OUT_DIR):
        os.makedirs(OUT_DIR)
    with open(SURVEY, encoding="utf-8") as handle:
        survey = json.load(handle)
    result = {"generated": datetime.now().isoformat(timespec="seconds"), "rows": [], "files": []}
    for direction, entries in (("register", survey.get("register", [])), ("send", survey.get("send", []))):
        for entry in entries:
            caller = int(entry["caller"], 16) if entry.get("caller") else None
            ids = ", ".join("%s %s" % (m["family"], m["name"]) for m in entry["matched_ids"])
            calc = []
            if caller:
                dump("%s_%s" % (direction, entry["caller_name"]), caller, result["files"])
            for ref in entry.get("referenced_functions", []):
                ea = int(ref["ea"], 16)
                if ea == caller:
                    continue
                dump("%s_ref" % direction, ea, result["files"])
                calc.append(ref["name"])
            result["rows"].append({
                "direction": direction,
                "ids": ids,
                "caller": entry["caller_name"],
                "caller_ea": entry["caller"],
                "referenced": sorted(set(calc)),
                "call": entry["call"],
            })
    path = os.path.join(OUT_DIR, "registry-table.json")
    with open(path, "w", encoding="utf-8") as handle:
        json.dump(result, handle, indent=2, ensure_ascii=False)
    print("=== id -> function table ===")
    for row in result["rows"]:
        print("%-9s %-34s %-14s refs=%s" % (row["direction"], row["ids"], row["caller"],
                                            ",".join(row["referenced"])[:60]))
    print("files=%d wrote %s" % (len(result["files"]), path))


if __name__ == "__main__":
    main()

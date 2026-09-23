#!/usr/bin/env python3
"""X7 probe, pass 2: scope the 144-byte NOTI2657 consumer.

Pass 1 (same file's first version) established:

    sub_1424FDF70 (NOTI2657):  memset(buf,0,144); sub_146EA0BE0(buf,144);
                               sub_142ABF650(qword_14E683C40, buf)
    sub_142ABF650(a1, a2):     key = sub_142AB29C0(a1);
                               node = rbtree_lookup(*(a1+312), key);
                               obj  = node[5];
                               obj && (*(vtable_of(obj) + 1136))(obj, a2)
    sub_142AB29C0(a1):         returns a small integer, 7/10/17/20/21/27/28 for
                               the first seven cases, otherwise a value derived
                               from the same "locale" accessor.

Two things follow and both need checking before this is called a game-state
channel:

  * the lookup key looks language/locale-derived, not a player or phase id, so
    the map at +312 may be a per-language handler table rather than a phase
    registry;
  * the object's class is still unknown, and only its class can say what the
    144 bytes mean.

Pass 2 therefore asks the cheap structural questions instead of scanning for
constants: who else calls the dispatcher, and who ever touches the global? A
dispatcher with exactly one caller (the NOTI handler) and a global nobody else
reads is a very different story from a subsystem with many call sites.

Run inside IDA (PYTHONHOME/PYTHONPATH must be cleared):

    idat -A -L<log> -S"analysis/dumps/ida_phase_x7b.py" <work>.i64
"""

import json
import os
from datetime import datetime

import ida_funcs
import ida_hexrays
import idautils
import idc

OUT_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "phase-x7b")

NOTI2657_HANDLER = 0x1424FDF70
PHASE_DISPATCH = 0x142ABF650
PHASE_KEY = 0x142AB29C0
PHASE_GLOBAL = 0x14E683C40

# Decompiling every caller of a global can explode; the structural question is
# answered by the first handful of distinct functions.
MAX_FUNCS = 30
MAX_LINES = 900

ANCHORS = [
    ("noti2657_handler", NOTI2657_HANDLER),
    ("phase_dispatch", PHASE_DISPATCH),
    ("phase_key", PHASE_KEY),
    ("phase_global", PHASE_GLOBAL),
]


def text_of(ea):
    for getter in ("GetDisasm", "generate_disasm_line"):
        fn = getattr(idc, getter, None)
        if fn is None:
            continue
        try:
            line = fn(ea)
            if line:
                return line
        except Exception:  # noqa: BLE001 - try the next spelling
            continue
    return ""


def func_of(ea):
    f = ida_funcs.get_func(ea)
    if f is None:
        return None, ""
    return f, ida_funcs.get_func_name(f.start_ea)


def xref_rows(target):
    rows = []
    for xref in idautils.XrefsTo(target, 0):
        f, name = func_of(xref.frm)
        rows.append(
            {
                "from": hex(xref.frm),
                "dir": "to" if xref.iscode else "data",
                "func": name,
                "func_ea": hex(f.start_ea) if f else None,
                "disasm": text_of(xref.frm),
            }
        )
    return rows


def main():
    if not os.path.isdir(OUT_DIR):
        os.makedirs(OUT_DIR)
    summary = {"generated": datetime.now().isoformat(timespec="seconds")}

    structural = {}
    callers = {}
    for label, ea in ANCHORS:
        rows = xref_rows(ea)
        structural[label] = {"ea": hex(ea), "count": len(rows), "refs": rows}
        for row in rows:
            if row["func_ea"]:
                callers.setdefault(row["func_ea"], row["func"])
        print("[xrefs] %s %#x -> %d" % (label, ea, len(rows)))
    summary["xrefs"] = structural
    summary["touching_functions"] = [
        {"ea": k, "name": v} for k, v in sorted(callers.items())
    ]

    targets = []
    seen = set()
    for label, ea in ANCHORS:
        if label == "phase_global":
            continue
        seen.add(ea)
        targets.append((label, ea))
    for ea_s, name in sorted(callers.items()):
        ea = int(ea_s, 16)
        if ea in seen or len(targets) >= MAX_FUNCS:
            continue
        seen.add(ea)
        targets.append(("touches_%s" % name.replace(".", "_"), ea))

    decompiled = []
    for label, ea in targets:
        record = {"label": label, "ea": hex(ea), "func_name": ida_funcs.get_func_name(ea)}
        try:
            cfunc = ida_hexrays.decompile(ea)
            text = str(cfunc) if cfunc else ""
        except Exception as exc:  # noqa: BLE001 - report, never abort the batch
            text = "hexrays failed: %s" % exc
        lines = text.splitlines()
        record["lines"] = len(lines)
        if len(lines) > MAX_LINES:
            lines = lines[:MAX_LINES] + ["... (%d more)" % (record["lines"] - MAX_LINES)]
        path = os.path.join(OUT_DIR, "%s_%x.c" % (label, ea))
        with open(path, "w", encoding="utf-8") as handle:
            handle.write("\n".join(lines) + "\n")
        record["file"] = path
        decompiled.append(record)
        print("[decompile] %s %#x lines=%d" % (label, ea, record["lines"]))
    summary["decompiled"] = decompiled

    path = os.path.join(OUT_DIR, "summary.json")
    with open(path, "w", encoding="utf-8") as handle:
        json.dump(summary, handle, indent=2, ensure_ascii=False)
    print("wrote %s" % path)


if __name__ == "__main__":
    main()

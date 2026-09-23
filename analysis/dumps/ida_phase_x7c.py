#!/usr/bin/env python3
"""X7 probe, pass 3: structural relations only. No decompilation.

Pass 2 collected the same information but also decompiled every caller, so a
single slow function could stall the run for many minutes and the xref counts
were only written at the very end. That is the wrong shape for a cheap question
(see the "low-signal probe" note in the pe-static-reverse skill).

This pass writes its JSON immediately after the xref walk and exits. Run it
first; only bring Hex-Rays in once the call-site counts say it is worth it.

    idat -A -L<log> -S"analysis/dumps/ida_phase_x7c.py" <work>.i64
"""

import json
import os
from datetime import datetime

import ida_funcs
import idautils
import idc

OUT_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "phase-x7c")

ANCHORS = [
    ("noti2657_handler", 0x1424FDF70),
    ("phase_dispatch", 0x142ABF650),
    ("phase_key", 0x142AB29C0),
    ("phase_global", 0x14E683C40),
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


def name_of(ea):
    f = ida_funcs.get_func(ea)
    if f is None:
        return None, ""
    return hex(f.start_ea), ida_funcs.get_func_name(f.start_ea)


def main():
    if not os.path.isdir(OUT_DIR):
        os.makedirs(OUT_DIR)
    summary = {"generated": datetime.now().isoformat(timespec="seconds"), "anchors": {}}

    per_func = {}
    for label, ea in ANCHORS:
        rows = []
        funcs = set()
        for xref in idautils.XrefsTo(ea, 0):
            func_ea, name = name_of(xref.frm)
            if func_ea:
                funcs.add((func_ea, name))
            rows.append(
                {
                    "from": hex(xref.frm),
                    "func": name,
                    "func_ea": func_ea,
                    "disasm": text_of(xref.frm),
                }
            )
        summary["anchors"][label] = {
            "ea": hex(ea),
            "ref_count": len(rows),
            "func_count": len(funcs),
            "funcs": sorted(n for _, n in funcs),
            "refs": rows,
        }
        for func_ea, name in funcs:
            per_func.setdefault(func_ea, {"name": name, "reaches": []})
            per_func[func_ea]["reaches"].append(label)
        print("[%s] %#x refs=%d funcs=%d" % (label, ea, len(rows), len(funcs)))

    summary["functions_by_anchor_reach"] = [
        {"func_ea": k, "name": v["name"], "reaches": sorted(v["reaches"])}
        for k, v in sorted(per_func.items())
    ]

    path = os.path.join(OUT_DIR, "summary.json")
    with open(path, "w", encoding="utf-8") as handle:
        json.dump(summary, handle, indent=2, ensure_ascii=False)
    print("wrote %s" % path)


if __name__ == "__main__":
    main()

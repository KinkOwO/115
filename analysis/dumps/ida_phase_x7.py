#!/usr/bin/env python3
"""X7 probe: what is the 144-byte NOTI2657 phase block?

Context (analysis/tasks/next71-legion-p4-truth-pipeline.md §8):

    NOTI2657 handler sub_1424FDF70:
        memset(buf, 0, 144)
        sub_146EA0BE0(buf, 144)          # read 144 bytes off the packet cursor
        sub_142ABF650(qword_14E683C40, buf)

`sub_142ABF650` looks a key up in a map (reported as obj+312) and then issues a
virtual call at "vtable + 1136" on the object it finds. The object's class is
what actually interprets the 144 bytes, so reading that virtual method gives the
field order -- which is what has to be known before the server may send 2657.

Three probes, cheapest first:

  A. scan the whole image for instructions carrying the immediate 0x470 (1136).
     Only the dispatcher and the consumer should use that offset, so the hit list
     is short and names the class family directly.
  B. scan for 0x90 (144) used as a size next to a memcpy/memset call -- the
     client never *builds* this block (it is server-authoritative), so this probe
     is expected to come back empty; an empty result is itself evidence that the
     layout must be read off the consumer.
  C. decompile the dispatcher, its key builder, and everything probe A found.

Run inside IDA (PYTHONHOME/PYTHONPATH must be cleared):

    idat -A -L<log> -S"analysis/dumps/ida_phase_x7.py" <work>.i64
"""

import json
import os
from datetime import datetime

import ida_bytes
import ida_funcs
import ida_hexrays
import ida_ida
import ida_idaapi
import ida_search
import idautils
import idc

OUT_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "phase-x7")

# Fixed anchors from the earlier passes (next65 / next71).
NOTI2657_HANDLER = 0x1424FDF70
PHASE_DISPATCH = 0x142ABF650
PHASE_KEY = 0x142AB29C0
PHASE_GLOBAL = 0x14E683C40

# Probe A: offsets that identify the phase consumer.
IMM_SCAN = {
    "vtable_slot_1136": 0x470,
    "map_field_312": 312,
}

MAX_HITS = 400
MAX_LINES = 1200


def func_of(ea):
    f = ida_funcs.get_func(ea)
    if f is None:
        return None, ""
    return f, ida_funcs.get_func_name(f.start_ea)


def _first(found):
    """IDA 9.4's find_imm hands back (ea, operand) while older builds return a
    bare ea; normalise both."""
    if isinstance(found, tuple):
        return found[0]
    return found


def find_immediate(value, limit=MAX_HITS):
    """Every instruction address whose operands encode `value` as an immediate
    or a displacement. Uses IDA's own search, which is far faster than walking
    every function item by item."""
    hits = []
    seen = set()
    ea = ida_ida.inf_get_min_ea()
    end = ida_ida.inf_get_max_ea()
    while ea < end and len(hits) < limit:
        try:
            found = _first(
                ida_search.find_imm(
                    ea, ida_search.SEARCH_NEXT | ida_search.SEARCH_DOWN, value
                )
            )
        except Exception as exc:  # noqa: BLE001 - a broken probe must not look like a clean scan
            print("[scan] %#x aborted at %#x: %s" % (value, ea, exc))
            hits.append(None)
            break
        if found == ida_idaapi.BADADDR or found is None or found >= end:
            break
        if found in seen:
            ea = found + 1
            continue
        seen.add(found)
        hits.append(found)
        ea = found + 1
    return [h for h in hits if h is not None]


def text_of(ea):
    """Disassembly text for one instruction, tolerant of the IDA build."""
    for getter in ("GetDisasm", "generate_disasm_line"):
        fn = getattr(idc, getter, None)
        if fn is None:
            continue
        try:
            line = fn(ea)
            if line:
                return line
        except Exception:  # noqa: BLE001 - fall through to the next spelling
            continue
    return ""


def describe(ea):
    f, name = func_of(ea)
    return {
        "ea": hex(ea),
        "func": name,
        "func_ea": hex(f.start_ea) if f else None,
        "disasm": text_of(ea),
    }


def decompile(label, ea, out):
    record = {"label": label, "ea": hex(ea), "func_name": ida_funcs.get_func_name(ea)}
    try:
        cfunc = ida_hexrays.decompile(ea)
        text = str(cfunc) if cfunc else ""
    except Exception as exc:  # noqa: BLE001 - report, never abort the batch
        text = "hexrays failed: %s" % exc
    lines = text.splitlines()
    record["lines"] = len(lines)
    if len(lines) > MAX_LINES:
        lines = lines[:MAX_LINES] + ["... (%d more lines)" % (record["lines"] - MAX_LINES)]
    path = os.path.join(OUT_DIR, "%s_%x.c" % (label, ea))
    with open(path, "w", encoding="utf-8") as handle:
        handle.write("\n".join(lines) + "\n")
    record["file"] = path
    out.append(record)
    print("[decompile] %s %#x %s lines=%d" % (label, ea, record["func_name"], record["lines"]))


def main():
    if not os.path.isdir(OUT_DIR):
        os.makedirs(OUT_DIR)
    summary = {"generated": datetime.now().isoformat(timespec="seconds")}

    # ---- probe A: distinctive offsets -------------------------------------
    scan = {}
    funcs = {}
    for label, value in IMM_SCAN.items():
        hits = [describe(ea) for ea in find_immediate(value)]
        scan[label] = {"value": value, "count": len(hits), "hits": hits}
        for h in hits:
            if h["func_ea"]:
                funcs.setdefault(h["func_ea"], h["func"])
        print("[scan] %s = %#x -> %d hits" % (label, value, len(hits)))
    summary["immediate_scan"] = scan
    summary["funcs_touching_phase_offsets"] = [
        {"ea": k, "name": v} for k, v in sorted(funcs.items())
    ]

    # ---- probe B: which functions reference the phase global --------------
    grefs = []
    for xref in idautils.XrefsTo(PHASE_GLOBAL, 0):
        f, name = func_of(xref.frm)
        grefs.append(
            {
                "from": hex(xref.frm),
                "func": name,
                "func_ea": hex(f.start_ea) if f else None,
                "type": int(xref.type),
                "disasm": text_of(xref.frm),
            }
        )
    summary["phase_global_xrefs"] = grefs
    print("[xrefs] qword_%x -> %d refs" % (PHASE_GLOBAL, len(grefs)))
    for g in grefs:
        if g["func_ea"]:
            funcs.setdefault(g["func_ea"], g["func"])

    # ---- probe C: decompile the anchors + every function probe A named ----
    targets = [
        ("noti2657_handler", NOTI2657_HANDLER),
        ("phase_dispatch", PHASE_DISPATCH),
        ("phase_key", PHASE_KEY),
    ]
    seen_ea = {ea for _, ea in targets}
    for ea_s, name in sorted(funcs.items()):
        ea = int(ea_s, 16)
        if ea in seen_ea:
            continue
        seen_ea.add(ea)
        targets.append(("hit_%s" % name.replace(".", "_"), ea))

    out = []
    for label, ea in targets:
        decompile(label, ea, out)
    summary["decompiled"] = out

    path = os.path.join(OUT_DIR, "summary.json")
    with open(path, "w", encoding="utf-8") as handle:
        json.dump(summary, handle, indent=2, ensure_ascii=False)
    print("wrote %s" % path)


if __name__ == "__main__":
    main()

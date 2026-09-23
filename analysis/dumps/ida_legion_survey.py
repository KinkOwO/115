#!/usr/bin/env python3
"""IDA Pro survey for the legion / apocalypse packet family.

Run inside IDA (File > Script file…) with client/DFO.exe.i64 loaded:

    idat64 -A -S"analysis/dumps/ida_legion_survey.py" client/DFO.exe.i64

or interactively: File > Script file… > this file.

What it does, per opcode, using the addresses already exported to
analysis/dumps/opcodes.tsv (string_va and table_slot_va):
  1. reads the pointer stored in the handler table slot (table_slot_va)
  2. names that pointer Handler_<family>_<name> when it is not named yet
  3. resolves the enum name string and lists its xrefs (registration sites)
  4. prints the pseudocode (hexrays) or the disassembly of the handler
  5. lists the first level callees, which is where the packet reader lives

It also dumps the two client addresses the fix notes call out:
  0x1406afcc0  world state gate that requires town state 1
  0x1406aae00  revive coin label selection

Output: analysis/dumps/legion-survey/{survey.json,<name>.txt}

The script is read-only apart from optional renaming; it never rewrites the
database, so save explicitly (or not at all) after reviewing the output.
"""

import json
import os
from datetime import datetime

try:
    import idaapi
    import idc
    import ida_bytes
    import ida_funcs
    import ida_name
    import ida_xref
    import idautils
except ImportError as exc:  # pragma: no cover - only runs inside IDA
    raise SystemExit("this script must run inside IDA: %s" % exc)

try:
    import ida_hexrays

    HAVE_HEXRAYS = True
except ImportError:
    HAVE_HEXRAYS = False

# (family, id, hex, enum name, name string VA, handler table slot VA)
# Copied from analysis/dumps/opcodes.tsv so the survey is self contained.
FAMILY = [
    ("cmd", 2043, "0x07FB", "LEGION_START", 0x14B06BF60, 0x14EF3CF38),
    ("cmd", 2044, "0x07FC", "LEGION_FAIL", 0x14B06BFA0, 0x14EF3CF40),
    ("cmd", 2045, "0x07FD", "LEGION_ENTER_DUNGEON", 0x14B06BFE0, 0x14EF3CF48),
    ("cmd", 2046, "0x07FE", "LEGION_REWARD_END", 0x14B06C030, 0x14EF3CF50),
    ("cmd", 2290, "0x08F2", "VENUS_OPERATION_SELECT", 0x14B071D70, 0x14EF3D6F0),
    ("cmd", 2293, "0x08F5", "VENUS_END_AT_PHASE4", 0x14B071E80, 0x14EF3D708),
    ("cmd", 2354, "0x0932", "LEGION_OPERATION_SELECT", 0x14B073620, 0x14EF3D8F0),
    ("cmd", 2355, "0x0933", "APOCALYPSE_ROLE_SELECT", 0x14B073680, 0x14EF3D8F8),
    ("cmd", 2424, "0x0978", "LEGION_ACHIEVEMENT_ACTION", 0x14B0750F0, 0x14EF3DB20),
    ("noti", 1474, "0x05C2", "DUNGEON_TIMEOUT_TIME", 0x14B016C20, 0x14EF362C0),
    ("noti", 2252, "0x08CC", "LEGION_BASIC_CLEAR_REWARD", 0x14B02ED50, 0x14EF37B10),
    ("noti", 2253, "0x08CD", "LEGION_ADDITIONAL_CLEAR_REWARD", 0x14B02EDB0, 0x14EF37B18),
    ("noti", 2254, "0x08CE", "LEGION_ENTRY_CHARAC_INFO", 0x14B02EE20, 0x14EF37B20),
    ("noti", 2568, "0x0A08", "PREPARE_LEGION_ENTER_DUNGEON", 0x14B036880, 0x14EF384F0),
    ("noti", 2657, "0x0A61", "LEGION_PHASE_CLEAR_TICK", 0x14B038AD0, 0x14EF387B8),
    ("noti", 2895, "0x0B4F", "LEGION_INFO", 0x14B03E900, 0x14EF38F28),
    ("noti", 2896, "0x0B50", "LEGION_OPERATION", 0x14B03E940, 0x14EF38F30),
]

# Addresses named by the fix notes; dump them even when unnamed.
EXTRA_TARGETS = [
    (0x1406AFCC0, "world state gate (expects town state 1)"),
    (0x1406AAE00, "revive coin label selection"),
]

OUT_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "legion-survey")


def ensure_dir():
    if not os.path.isdir(OUT_DIR):
        os.makedirs(OUT_DIR)


def read_ptr(ea):
    """Read an 8 byte pointer; return None when the address is not mapped."""
    value = ida_bytes.get_qword(ea)
    if value in (0, idc.BADADDR):
        return None
    if not ida_bytes.is_loaded(value):
        return None
    return value


def name_of(ea):
    return ida_funcs.get_func_name(ea) or ida_name.get_name(ea) or ""


def rename_handler(ea, family, name):
    """Give an unnamed handler a stable name so later runs are comparable."""
    if name_of(ea):
        return False
    return bool(ida_name.set_name(ea, "Handler_%s_%s" % (family.upper(), name), ida_name.SN_NOWARN))


def pseudocode(ea, limit=400):
    if not HAVE_HEXRAYS:
        return None
    try:
        func = ida_funcs.get_func(ea)
        if func is None:
            return None
        cfunc = ida_hexrays.decompile(func.start_ea)
    except Exception as exc:  # decompiler failures are reported, not fatal
        return "hexrays failed: %s" % exc
    if cfunc is None:
        return None
    text = str(cfunc)
    lines = text.splitlines()
    if len(lines) > limit:
        lines = lines[:limit] + ["... (%d more lines)" % (len(text.splitlines()) - limit)]
    return "\n".join(lines)


def disassembly(ea, limit=120):
    lines = []
    func = ida_funcs.get_func(ea)
    if func is None:
        return "not a function: %#x" % ea
    for item in idautils.FuncItems(func.start_ea):
        lines.append("%#x  %s" % (item, idc.generate_disasm_line(item, 0)))
        if len(lines) >= limit:
            lines.append("... (truncated)")
            break
    return "\n".join(lines)


def callees(ea):
    func = ida_funcs.get_func(ea)
    if func is None:
        return []
    out = []
    seen = set()
    for item in idautils.FuncItems(func.start_ea):
        for target in idautils.CodeRefsFrom(item, 0):
            target_func = ida_funcs.get_func(target)
            if target_func is None or target_func.start_ea == func.start_ea:
                continue
            if target_func.start_ea in seen:
                continue
            seen.add(target_func.start_ea)
            out.append({"ea": hex(target_func.start_ea), "name": name_of(target_func.start_ea)})
    return out


def xrefs_of_string(ea):
    out = []
    for xref in idautils.DataRefsTo(ea):
        func = ida_funcs.get_func(xref)
        out.append({"ea": hex(xref), "func": hex(func.start_ea) if func else "", "name": name_of(xref)})
    return out


def survey_opcode(entry):
    family, opcode, hexcode, name, string_va, slot_va = entry
    record = {
        "family": family,
        "opcode": opcode,
        "hex": hexcode,
        "name": name,
        "string_va": hex(string_va),
        "table_slot_va": hex(slot_va),
    }
    text = ida_bytes.get_strlit_contents(string_va, -1, 0)
    record["enum_string"] = text.decode("utf-8", "replace") if text else ""
    record["string_xrefs"] = xrefs_of_string(string_va)
    handler = read_ptr(slot_va)
    if handler is None:
        record["handler"] = ""
        record["note"] = "table slot is empty or unreadable"
        return record, None
    record["handler"] = hex(handler)
    record["renamed"] = rename_handler(handler, family, name)
    record["handler_name"] = name_of(handler)
    record["pseudocode"] = pseudocode(handler)
    record["callees"] = callees(handler)
    return record, handler


def survey_extra(ea, note):
    record = {"ea": hex(ea), "note": note, "name": name_of(ea)}
    record["pseudocode"] = pseudocode(ea, limit=250)
    record["callees"] = callees(ea)
    return record, ea


def main():
    ensure_dir()
    survey = {
        "generated": datetime.now().isoformat(timespec="seconds"),
        "input_file": idaapi.get_input_file_path(),
        "image_base": hex(idaapi.get_imagebase()),
        "hexrays": HAVE_HEXRAYS,
        "opcodes": [],
        "extra": [],
    }
    for entry in FAMILY:
        record, handler = survey_opcode(entry)
        survey["opcodes"].append(record)
        body = record.get("pseudocode") or ""
        if handler is not None and not body:
            body = disassembly(handler)
        path = os.path.join(OUT_DIR, "%s_%s.txt" % (entry[0], entry[3].lower()))
        with open(path, "w", encoding="utf-8") as handle:
            handle.write("%s %s %s handler=%s\n\n" % (entry[0], entry[2], entry[3], record.get("handler", "")))
            handle.write(body or "(no body: handler unreadable)")
            handle.write("\n\ncallees:\n")
            for callee in record.get("callees", []):
                handle.write("  %s %s\n" % (callee["ea"], callee["name"]))
        print("[opcode] %-5s %-6s handler=%s body=%d bytes -> %s"
              % (entry[0], entry[3], record.get("handler", "-"), len(body), path))
    for ea, note in EXTRA_TARGETS:
        record, _ = survey_extra(ea, note)
        survey["extra"].append(record)
        body = record.get("pseudocode") or ""
        if not body:
            body = disassembly(ea)
        path = os.path.join(OUT_DIR, "extra_%x.txt" % ea)
        with open(path, "w", encoding="utf-8") as handle:
            handle.write("%#x  %s\n\n%s" % (ea, note, body))
        print("[extra ] %#x %s body=%d bytes -> %s" % (ea, note, len(body), path))
    manifest = os.path.join(OUT_DIR, "survey.json")
    with open(manifest, "w", encoding="utf-8") as handle:
        json.dump(survey, handle, indent=2, ensure_ascii=False)
    print("wrote %s" % manifest)


if __name__ == "__main__":
    main()

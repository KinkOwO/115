#!/usr/bin/env python3
"""IDA helper: list and decompile every function that touches a global.

Set via environment variables:

    IDA_GLOBALS    comma separated addresses, e.g. "0x14E683C40,0x14E683C78"
    IDA_OUT_DIR    output directory (default <cwd>/global-users)
    IDA_MAX_LINES  per-function line cap (default 900)
    IDA_MAX_FUNCS  decompile at most N distinct functions per global (default 25)

Output per global: `g<VA>_f<count>.c` for each distinct containing function,
plus `summary.json`.
"""

import json
import os
import sys

import ida_auto
import ida_bytes
import ida_funcs
import ida_hexrays

import idautils
import idc


def env(name, default=""):
    return os.environ.get(name, default)


def pseudocode(ea, max_lines):
    try:
        cf = ida_hexrays.decompile(ea)
    except Exception:
        cf = None
    if cf is None:
        return None
    # IDA 9.x: str(cfunc) is the supported way to get the listing;
    # ida_lines.tag_remove() expects a str and raises on the newer line objects.
    text = str(cf) if cf else ""
    out = text.splitlines()
    if len(out) > max_lines:
        out = out[:max_lines] + ["// ... (%d more lines)" % (len(out) - max_lines)]
    return "\n".join(out)


def disassembly(ea, max_lines):
    fn = ida_funcs.get_func(ea)
    if fn is None:
        return None
    out = []
    for item in idautils.FuncItems(fn.start_ea):
        out.append("%#x  %s" % (item, idc.generate_disasm_line(item, 0)))
        if len(out) >= max_lines:
            break
    return "\n".join(out)


def main():
    raw = env("IDA_GLOBALS")
    if not raw:
        print("[globals] IDA_GLOBALS is required", file=sys.stderr)
        idc.qexit(1)
        return
    out_dir = env("IDA_OUT_DIR") or os.path.join(os.getcwd(), "global-users")
    max_lines = int(env("IDA_MAX_LINES", "900"))
    max_funcs = int(env("IDA_MAX_FUNCS", "25"))
    os.makedirs(out_dir, exist_ok=True)

    ida_auto.auto_wait()
    have_hexrays = ida_hexrays.init_hexrays_plugin()

    summary = {"hexrays": bool(have_hexrays), "globals": []}
    for token in raw.split(","):
        token = token.strip()
        if not token:
            continue
        va = int(token, 0)
        entry = {"global": va, "name": idc.get_name(va), "functions": []}
        seen = set()
        for xref in idautils.XrefsTo(va, 0):
            fn = ida_funcs.get_func(xref.frm)
            if fn is None or fn.start_ea in seen:
                continue
            seen.add(fn.start_ea)
        order = sorted(seen)
        for idx, start in enumerate(order[:max_funcs]):
            name = ida_funcs.get_func_name(start) or ("sub_%X" % start)
            text = pseudocode(start, max_lines) if have_hexrays else None
            mode = "pseudocode"
            if text is None:
                text = disassembly(start, max_lines)
                mode = "disassembly"
            if text is None:
                continue
            path = os.path.join(out_dir, "g%x_f%d_%s_%x.c" % (va, idx, name, start))
            with open(path, "w", encoding="utf-8") as fh:
                fh.write("// global %#x (%s) used by %s @%#x [%s]\n\n"
                         % (va, entry["name"], name, start, mode))
                fh.write(text + "\n")
            entry["functions"].append({"ea": start, "name": name, "mode": mode,
                                       "file": path})
            print("[globals] %#x used-by %-24s %#x  %s" % (va, name, start, mode))
        entry["total_functions"] = len(order)
        summary["globals"].append(entry)

    with open(os.path.join(out_dir, "summary.json"), "w", encoding="utf-8") as fh:
        json.dump(summary, fh, indent=2, ensure_ascii=False, default=str)
    print("[globals] wrote %s" % out_dir)
    idc.qexit(0)


if __name__ == "__main__":
    main()

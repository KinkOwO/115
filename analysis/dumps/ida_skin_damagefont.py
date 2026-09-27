#!/usr/bin/env python3
"""Locate the manager that registers the damage-font skin packets and decompile
their handlers. The skin-cargo manager sub_1444E81C0 only covers 1545/1546/1547/
1565/1671/1672/2243/2426/2641. The damage-font family lives elsewhere:

  noti 1231 DAMAGE_FONT_SKIN_LIST
  cmd  1282 SELECT_DAMAGE_FONT_SKIN
  cmd  1592 MAKE_SKIN
  cmd  857  OPEN_AURA_SKIN_SLOT
  cmd  2155 USE_WEAPON_EFFECT_SKIN_EXTRACTOR
  noti 5056 USE_WEAPON_EFFECT_SKIN_REGISTER

Registrars: sub_14599D5D0 (NOTI), sub_14599D450 (CMD), shape (mgr, id, handler, 0).
Hex-Rays may render the id immediate as decimal OR hex, so match both.

Run:  idat -A -L<log> -S"ida_skin_damagefont.py" <work>/DFO.exe.i64
"""
import os
import re
import json
import ida_funcs
import ida_hexrays
import idautils
import idc

NOTI_HELPER = 0x14599D5D0
CMD_HELPER = 0x14599D450

TARGETS = {
    1231: "noti1231_damage_font_skin_list",
    1282: "cmd1282_select_damage_font_skin",
    1592: "cmd1592_make_skin",
    857: "cmd857_open_aura_skin_slot",
    2155: "cmd2155_use_weapon_effect_skin_extractor",
    5056: "noti5056_use_weapon_effect_skin_register",
}

OUT_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "skin-noti")
os.makedirs(OUT_DIR, exist_ok=True)

CALL_RE = re.compile(r"sub_14599D(?:5D0|450)\(([^;]{0,240})\);")


def parse_id(tok):
    tok = tok.strip().rstrip("u")
    try:
        if tok.lower().startswith("0x"):
            return int(tok, 16)
        return int(tok, 10)
    except ValueError:
        return None


def decompile_text(ea):
    try:
        cf = ida_hexrays.decompile(ea)
    except Exception as exc:
        return None, "hexrays failed: %r" % exc
    if cf is None:
        return None, "none"
    return str(cf), None


def main():
    callers = {}
    for helper in (NOTI_HELPER, CMD_HELPER):
        for ref in idautils.CodeRefsTo(helper, 0):
            ea = ref if isinstance(ref, int) else ref.frm
            f = ida_funcs.get_func(ea)
            if f is not None:
                callers.setdefault(f.start_ea, []).append(ea)
    print("registrar call sites in %d functions" % len(callers))

    found = {}
    for func_ea in sorted(callers):
        text, err = decompile_text(func_ea)
        if text is None:
            continue
        for m in CALL_RE.finditer(text):
            args = m.group(1)
            parts = [p.strip() for p in args.split(",")]
            if len(parts) < 3:
                continue
            pid = parse_id(parts[1])
            if pid in TARGETS:
                hname = re.search(r"(sub_[0-9A-Fa-f]+)", parts[2])
                found[pid] = {
                    "manager": "0x%X" % func_ea,
                    "handler": hname.group(1) if hname else parts[2],
                }
    print("FOUND %s" % json.dumps(found, indent=2))

    # Dump the manager constructor(s) that register these, plus each handler body.
    managers = sorted({v["manager"] for v in found.values()})
    for mgr in managers:
        text, err = decompile_text(int(mgr, 16))
        if text:
            p = os.path.join(OUT_DIR, "df_manager_%s.c" % mgr.lower())
            open(p, "w", encoding="utf-8").write(text)
            print("MGR %s -> %s lines=%d" % (mgr, p, text.count("\n")))

    for pid, label in TARGETS.items():
        if pid not in found:
            print("MISS %d %s" % (pid, label))
            continue
        h = found[pid]["handler"]
        hm = re.match(r"sub_([0-9A-Fa-f]+)", h)
        if not hm:
            print("HANDLER-NOT-SUB %d %s" % (pid, h))
            continue
        hea = int(hm.group(1), 16)
        f = ida_funcs.get_func(hea)
        start = f.start_ea if f else hea
        text, err = decompile_text(start)
        if text is None:
            print("FAIL %d %s %x %s" % (pid, label, start, err))
            continue
        p = os.path.join(OUT_DIR, "df_handler_%s_%x.c" % (label, start))
        open(p, "w", encoding="utf-8").write(text)
        print("OK %d %s %x lines=%d -> %s" % (pid, label, start, text.count("\n"), p))

    json.dump(found, open(os.path.join(OUT_DIR, "df-found.json"), "w"), indent=2)
    idc.qexit(0)


main()

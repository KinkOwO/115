#!/usr/bin/env python3
"""装备调适 CMD2258 (ENUM_CMDPACKET_EQUIPMENT_AWAKENING, 0x8D2) 取证轮。

三条互不相关的证据线，全部落盘，便于离线复核：

1. **立即数命中**：`.text` 里真含 2258 立即数的指令（C2S 发包点写 opcode、S2C 分支比较）。
2. **注册表命中**：`sub_14599D450` / `sub_1459A2FB0`（CMD 表）与
   `sub_14599D5D0` / `sub_1459A3DD0`（NOTI 表）调用点附近 ±64 字节内出现 2258 的位置
   ⇒ S2C handler 地址（技能 §4：不要用 find_imm 全盘扫）。
3. **命中函数反编译**：把 1/2 里出现的函数（去重、限体积）走 Hex-Rays，写 `.c` 便于人工核对字段宽度。

产出目录：`analysis/dumps/awakening-2258/`
  - `hits.json`（原始命中）
  - `<n>_<VA>_<name>.c`（反编译）

运行（本机 IDA 在 D:\\tools\\ida94，必须先清 PYTHONPATH/PYTHONHOME）：
  cd /d/tools/ida94
  ./idat.exe -A -Ld:\\115us-backup\\ida-awakening-2258.log \
      -S"d:\\115us\\analysis\\dumps\\ida_awakening_2258.py" \
      "d:\\115us-backup\\ida-work\\DFO.exe.i64"
"""

import collections
import json
import os
from datetime import datetime

import ida_bytes
import ida_funcs
import ida_hexrays
import ida_segment
import ida_ua
import ida_xref
import idc

OPCODE = 2258
OPCODE_HEX = 0x8D2

REGISTER_FUNCS = {
    0x14599D450: "CMD register (id, handler)",
    0x1459A2FB0: "CMD table insert (table, id, handler)",
    0x14599D5D0: "NOTI register (id, handler, flags)",
    0x1459A3DD0: "NOTI table insert (table, id, handler, flags)",
}

OUT_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "awakening-2258")


def text_ranges():
    out = []
    for i in range(ida_segment.get_segm_qty()):
        seg = ida_segment.getnseg(i)
        if seg is None:
            continue
        name = ida_segment.get_segm_name(seg)
        if name and name.startswith(".text"):
            out.append((seg.start_ea, seg.end_ea, name))
    return out


def imm_insn(ea, value):
    """返回 (指令地址, 助记符, 文本)，要求该指令里真的有这个立即数操作数。"""
    insn = ida_ua.insn_t()
    for start in range(max(0, ea - 7), ea + 1):
        if ida_ua.decode_insn(insn, start) > 0:
            for op in insn.ops:
                if op.type == ida_ua.o_imm and op.value == value:
                    return start, idc.print_insn_mnem(start), idc.GetDisasm(start)
    return None


def func_key(ea):
    f = ida_funcs.get_func(ea)
    if not f:
        return None, "(no func)"
    return f.start_ea, "%s" % ida_funcs.get_func_name(f.start_ea)


def scan_immediates():
    per_func = collections.defaultdict(list)
    raw = 0
    pat = OPCODE.to_bytes(4, "little")
    for lo, hi, _ in text_ranges():
        start = lo
        while True:
            ea = ida_bytes.find_bytes(pat, start, hi - start)
            if ea is None or ea == idc.BADADDR or ea >= hi:
                break
            start = ea + 1
            raw += 1
            got = imm_insn(ea, OPCODE)
            if got is None:
                continue
            ins_ea, mn, text = got
            fea, fname = func_key(ins_ea)
            key = "0x%X %s" % (fea, fname) if fea else "(no func)"
            per_func[key].append({"ea": "0x%X" % ins_ea, "mn": mn, "asm": text})
    return raw, per_func


def scan_registrations():
    """扫注册函数的调用点，找附近带 2258 的（= 该 opcode 的 handler 注册点）。"""
    out = {}
    for reg_ea, label in REGISTER_FUNCS.items():
        hits = []
        # 枚举调用点（技能 §4：不要 find_imm 全盘扫）
        xr = ida_xref.get_first_cref_to(reg_ea)
        while xr != idc.BADADDR:
            hits.append(xr)
            xr = ida_xref.get_next_cref_to(reg_ea, xr)
        near = []
        for call_ea in hits:
            f = ida_funcs.get_func(call_ea)
            if not f:
                continue
            lo = max(f.start_ea, call_ea - 64)
            hi = min(f.end_ea, call_ea + 64)
            found = []
            for a in range(lo, hi):
                insn = ida_ua.insn_t()
                if ida_ua.decode_insn(insn, a) > 0:
                    for op in insn.ops:
                        if op.type == ida_ua.o_imm and op.value == OPCODE:
                            found.append({"ea": "0x%X" % a, "asm": idc.GetDisasm(a)})
            if found:
                fea, fname = func_key(call_ea)
                near.append({
                    "call_site": "0x%X" % call_ea,
                    "func": "0x%X %s" % (fea, fname) if fea else "(no func)",
                    "asm": idc.GetDisasm(call_ea),
                    "imm_hits": found,
                })
        out[label] = {"calls_total": len(hits), "with_opcode": near}
    return out


def decompile_funcs(func_eas):
    if not ida_hexrays.init_hexrays_plugin():
        print("!! hexrays unavailable")
        return []
    written = []
    for idx, fea in enumerate(sorted(func_eas)):
        f = ida_funcs.get_func(fea)
        if not f:
            continue
        size = f.end_ea - f.start_ea
        if size > 0x30000:
            print("   skip big func 0x%X (%d bytes)" % (fea, size))
            continue
        try:
            cf = ida_hexrays.decompile(fea)
        except Exception as e:  # noqa: BLE001
            print("   decompile fail 0x%X: %s" % (fea, e))
            continue
        if cf is None:
            continue
        name = ida_funcs.get_func_name(fea)
        path = os.path.join(OUT_DIR, "%02d_%X_%s.c" % (idx, fea, name))
        body = str(cf)
        with open(path, "w", encoding="utf-8") as h:
            h.write("// %s  size=%d\n\n" % (name, size))
            h.write(body)
        written.append({"func": "0x%X" % fea, "name": name, "bytes": size, "file": os.path.basename(path)})
        print("   wrote %s (%d bytes)" % (path, size))
    return written


def main():
    if not os.path.isdir(OUT_DIR):
        os.makedirs(OUT_DIR)

    print("== 1. immediate scan ==")
    raw, per_func = scan_immediates()
    print("   raw bytes hits=%d funcs=%d" % (raw, len(per_func)))
    for k in sorted(per_func, key=lambda x: -len(per_func[x])):
        print("   %-58s hits=%d" % (k, len(per_func[k])))
        for r in per_func[k][:8]:
            print("        %-12s %s" % (r["ea"], r["asm"][:72]))

    print("== 2. registration scan ==")
    regs = scan_registrations()
    for label, data in regs.items():
        print("   %-40s calls=%d with_opcode=%d" % (label, data["calls_total"], len(data["with_opcode"])))
        for row in data["with_opcode"][:10]:
            print("        %s  %s" % (row["call_site"], row["asm"][:72]))
            for imm in row["imm_hits"][:4]:
                print("             imm %s  %s" % (imm["ea"], imm["asm"][:72]))

    funcs = set()
    for k in per_func:
        if k.startswith("0x"):
            funcs.add(int(k.split()[0], 16))
    for data in regs.values():
        for row in data["with_opcode"]:
            if row["func"].startswith("0x"):
                funcs.add(int(row["func"].split()[0], 16))

    print("== 3. decompile %d func(s) ==" % len(funcs))
    written = decompile_funcs(funcs)

    payload = {
        "generated": datetime.now().isoformat(timespec="seconds"),
        "opcode": OPCODE,
        "immediate_scan": {"raw_bytes_hits": raw, "by_func": {k: v for k, v in sorted(per_func.items())}},
        "registration_scan": regs,
        "decompiled": written,
    }
    jp = os.path.join(OUT_DIR, "hits.json")
    with open(jp, "w", encoding="utf-8") as h:
        json.dump(payload, h, indent=1, ensure_ascii=False)
    print("wrote %s" % jp)


if __name__ == "__main__":
    main()

#!/usr/bin/env python3
"""定位 qword_14EF2CA88/90/70/78 的写入函数，寻找虚表安装指令。

输出 ida-out/actor_vtable.json：
- 每个引用点所在函数名；
- 函数内所有 `lea rX, [rip+XXX]` 且 XXX 落在 .rdata 的候选；
- 候选虚表 +0x1F08 处若指向 .text，则列为命中。
"""
import json
import os

import ida_bytes
import ida_funcs
import ida_name
import ida_segment
import idautils
import idc

OUT_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "ida-out")
GLOBALS = [0x14EF2CA70, 0x14EF2CA78, 0x14EF2CA88, 0x14EF2CA90]


def main():
    os.makedirs(OUT_DIR, exist_ok=True)
    report = {"global": {hex(g): [] for g in GLOBALS}, "vtables": []}

    # 1) 收集引用各全局的代码位置与所在函数
    for g in GLOBALS:
        seen = set()
        for x in idautils.XrefsTo(g, 0):
            if not x.iscode:
                continue
            f = ida_funcs.get_func(x.frm)
            fname = ida_funcs.get_func_name(x.frm) if f else "?"
            if f.start_ea in seen:
                continue
            seen.add(f.start_ea)
            report["global"][hex(g)].append(
                {"ref": "%#x" % x.frm, "func": fname, "start": "%#x" % f.start_ea})

    # 2) 在这些函数里找虚表安装（lea rX, [rip+vt] 后 mov [reg], rX）
    funcs = set()
    for g in GLOBALS:
        for item in report["global"][hex(g)]:
            funcs.add(item["start"])
    text = ida_segment.get_segm_by_name(".text").start_ea
    rdata = ida_segment.get_segm_by_name(".rdata").start_ea
    rdata_end = ida_segment.get_segm_by_name(".rdata").end_ea
    hits = []
    for start_s in funcs:
        start = int(start_s, 16)
        f = ida_funcs.get_func(start)
        if f is None:
            continue
        for ea in idautils.FuncItems(f.start_ea):
            # lea rX, [rip+disp]
            mnem = idc.print_insn_mnem(ea)
            if mnem != "lea":
                continue
            for opr in range(2):
                if idc.get_operand_type(ea, opr) != idc.o_displ:
                    continue
                tgt = idc.get_operand_value(ea, opr)
                if tgt == idc.BADADDR or not (rdata <= tgt < rdata_end):
                    continue
                # 检查 [tgt + 0x1F08] 指向 .text
                ptr = ida_bytes.get_qword(tgt + 0x1F08)
                if ida_bytes.get_segm_name(ptr) == ".text":
                    hits.append({
                        "func": ida_funcs.get_func_name(f.start_ea),
                        "func_start": "%#x" % f.start_ea,
                        "insn": "%#x  %s" % (ea, idc.GetDisasm(ea)),
                        "vtable": "%#x" % tgt,
                        "getter_1f08": "%#x" % ptr,
                        "getter_name": ida_name.get_name(ptr) or "",
                    })
    report["vtables"] = hits
    path = os.path.join(OUT_DIR, "actor_vtable.json")
    with open(path, "w", encoding="utf-8") as handle:
        json.dump(report, handle, indent=2, ensure_ascii=False)
    print("globals ref funcs: %d" % len(funcs))
    print("vtable install hits: %d" % len(hits))
    for h in hits:
        print("  %s @ %s  vtable=%s getter=%s %s" % (
            h["func"], h["func_start"], h["vtable"], h["getter_1f08"], h["getter_name"]))
    print("wrote %s" % path)


if __name__ == "__main__":
    main()

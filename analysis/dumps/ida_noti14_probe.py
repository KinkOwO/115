#!/usr/bin/env python3
"""NOTI14 / toast 取证探针（在 IDA 批处理模式下运行于 IDB 工作副本）。

输出 analysis/dumps/ida-out/noti14_probe.json：
- 目标函数：NOTI14 handler、toast 判据、两条抑制分支、actor getter 的
  反编译伪代码 + 函数范围 + 调用者；
- 虚表槽 +0x1f08 解析尝试：若 IDB 类型信息可解析间接调用目标则直接给出；
- 模式字 [0x14E66C090 + 0x11b0] 写入者扫描。
"""
import json
import os

import ida_bytes
import ida_funcs
import ida_hexrays
import ida_name
import idautils
import idc

OUT_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "ida-out")

TARGETS = [
    ("noti14_handler", 0x1452E9810),
    ("toast_judge", 0x146CFDD20),
    ("suppress_branch_A", 0x1459AB2E0),
    ("suppress_branch_B", 0x145242380),
    ("actor_getter", 0x145EFAFB0),
]

GLOBAL_CMD_REGISTRY = 0x14E66C090
MODE_OFF = 0x11B0


def func_bounds(ea):
    f = ida_funcs.get_func(ea)
    return (f.start_ea, f.end_ea) if f else (None, None)


def callees(ea):
    out = []
    start, end = func_bounds(ea)
    if start is None:
        return out
    for item in idautils.FuncItems(start):
        for t in idautils.CodeRefsFrom(item, 0):
            tf = ida_funcs.get_func(t)
            if tf is not None and tf.start_ea != start:
                out.append("%#x %s" % (tf.start_ea, ida_funcs.get_func_name(tf.start_ea)))
    return sorted(set(out))


def callers(ea, cap=60):
    out = []
    for x in idautils.XrefsTo(ea, 0):
        if x.iscode:
            out.append("%#x %s" % (x.frm, ida_funcs.get_func_name(x.frm)))
    return out[:cap]


def decompile(ea):
    try:
        c = ida_hexrays.decompile(ea)
        return str(c) if c else "no pseudocode"
    except Exception as exc:
        return "hexrays failed: %s" % exc


def mode_word_writers():
    """扫描引用 qword_14E66C090 的代码，找出对 [+0x11b0] 的写与比较。"""
    hits = []
    for x in idautils.XrefsTo(GLOBAL_CMD_REGISTRY, 0):
        if not x.iscode:
            continue
        frm = x.frm
        f = ida_funcs.get_func(frm)
        if f is None:
            continue
        # 在 frm 所在函数里找对 [reg+0x11b0] 的 mov/cmp（reg 来自引用该全局的指令）
        for item in idautils.FuncItems(f.start_ea):
            text = idc.GetDisasm(item)
            if ("0x%X" % MODE_OFF) in text or ("0x%x" % MODE_OFF) in text or "11B0h" in text or "11B0h" in text.upper():
                hits.append("%#x  %s" % (item, text))
        if len(hits) > 400:
            break
    return sorted(set(hits))


def main():
    os.makedirs(OUT_DIR, exist_ok=True)
    report = {"targets": {}, "mode_word_writers": mode_word_writers()}
    for label, ea in TARGETS:
        start, end = func_bounds(ea)
        rec = {
            "ea": hex(ea),
            "name": ida_funcs.get_func_name(ea),
            "func_range": ("%#x-%#x" % (start, end)) if start else None,
            "pseudo": decompile(ea),
            "callees": callees(ea),
            "callers": callers(ea),
        }
        report["targets"][label] = rec
        print("[%s] %#x %s range=%s pseudo_lines=%d callers=%d"
              % (label, ea, rec["name"], rec["func_range"],
                 len(rec["pseudo"].splitlines()), len(rec["callers"])))
    path = os.path.join(OUT_DIR, "noti14_probe.json")
    with open(path, "w", encoding="utf-8") as handle:
        json.dump(report, handle, indent=2, ensure_ascii=False)
    print("wrote %s" % path)


if __name__ == "__main__":
    main()

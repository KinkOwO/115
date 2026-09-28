#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""df52: opcode 1551 到底是什么——点名它的接收处理器与处理器调用的界面事件。

实机 2026-09-28（`roles_persist_..._092434_869967_next37`）：按用户「按表情快捷键」的每一步时间戳
上，唯一成组出现的帧是 CMD 1551，体固定 8 字节 `u32 10179, u32 0`，01:25:56 / 01:26:04 /
01:26:29 / 01:26:55 / 01:27:00 五次，其余时段没有别的未登记 opcode。服务端对 1551 **零处理**
（`grep -rn '1551' cmd internal` 无命中）。df38 的注册表 dump 里 1551 有接收处理器：
`sub_14599D5D0(qword_14E66C090, 1551, sub_14449CDB0, 0)`（`df38_registrar_0x14449caf0.c:58`）
⇒ 客户端**能收** 1551，这正是「上行请求 + 服务端广播回来才画气泡」的形状。

本轮只反编译 `sub_14449CDB0` 与它调用的函数，看它把帧交给哪个界面事件（`sub_14668C520(单例, N, …)`
里那个 N），据此判断 1551 是不是表情气泡。只读轮。
"""
import os
import re

import ida_funcs
import ida_hexrays
import ida_name
import idautils
import idc

OUT = r"D:\115us\analysis\dumps\skin-noti"
HANDLER = 0x14449CDB0


def fname(ea):
    return ida_name.get_name(ea) or ""


def body_of(ea):
    try:
        return str(ida_hexrays.decompile(ea))
    except Exception:
        return ""


def dump(tag, text):
    name = "df52_%s.c" % re.sub(r"\W+", "_", tag)
    with open(os.path.join(OUT, name), "w", encoding="utf-8") as h:
        h.write("// %s\n\n%s\n" % (tag, text))
    return name


print("=== 1. call-site constants ===", flush=True)
b = body_of(HANDLER)
if not b:
    print("handler decompile empty", flush=True)
    raise SystemExit
print(dump("handler_1551", b), flush=True)
print(b[:14000], flush=True)

print("=== 2. store offsets ===", flush=True)
# 处理器自己的调用者：谁还会走到它（除注册表外有没有直接调用点）。
callers = [x.frm for x in idautils.XrefsTo(HANDLER, 0)]
print("xrefs to handler: %d" % len(callers), flush=True)
for xr in callers:
    f = ida_funcs.get_func(xr)
    print("  from %s in %s %s" % (hex(xr), hex(f.start_ea) if f else "-", fname(f.start_ea) if f else "-"), flush=True)

print("=== 3. parent/child calls ===", flush=True)
targets = []
for m in re.finditer(r"\b(sub_[0-9A-Fa-f]{6,})\(", b):
    ea = idc.get_name_ea_simple(m.group(1))
    if ea != idc.BADADDR and ea not in targets:
        targets.append(ea)
print("callees: %d" % len(targets), flush=True)
for ea in targets:
    n = sum(1 for x in idautils.XrefsTo(ea, 0) if idc.print_insn_mnem(x.frm) == "call")
    size = ida_funcs.get_func(ea).end_ea - ida_funcs.get_func(ea).start_ea if ida_funcs.get_func(ea) else 0
    print("  %s %s calls=%d size=%d" % (hex(ea), fname(ea), n, size), flush=True)
# 只反编译小函数，避免一次大函数把 Hex-Rays 卡住几十分钟。
for ea in targets:
    f = ida_funcs.get_func(ea)
    if not f or f.end_ea - f.start_ea > 0x600:
        continue
    cb = body_of(ea)
    if cb:
        print("  wrote " + dump("callee_%s" % fname(ea), cb), flush=True)
print("\n=== df52 done ===", flush=True)

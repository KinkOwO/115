#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""df53: 表情气泡的回包半——① 谁发 CMD 1551、发完挂了什么等待态；② 界面事件 2345 的消费者是谁。

df52 已闭环（`CLIENT-MECHANICS.md` §21）：
  * 实机 5 次按快捷键各产生一帧 CMD 1551，体 = `u32 10179, u32 0`，10179 是
    `configs/skin-storage-items.json` 里 `instant emoticon` 的皮肤 id ⇒ 上行体首是表情皮肤 id。
  * 客户端 1551 的接收处理器 `sub_14449CDB0` 读 `u8, u8, u32`，写 496 字节单例
    `qword_14E659EA8` 的 `+64+8*i` / `+68+8*i`（只有 (496-64)/8 = 54 格）⇒ 下行体是另一套字段，
    把上行体原样回声会让 i=0xC3=195 越界。
  * CMD 2039 的处理器 `sub_1444E8CC0` 一个体字节都不读，状态非 0 只发界面事件
    `sub_146694510(qword_14E683C78, 2345, -1, 0, 1)`。

本轮两段：§1 从 `sub_14449CAF0`（既是 1551 的登记点也是那个单例的 ctor）的调用者与
`qword_14E659EA8` 的引用者里找**发送方**（体里出现 1551 / 0x60F 的函数）；
§2 从 UI 单例 `qword_14E683C78` 的全部引用者里找伪代码中出现常量 2345 的函数，
把「谁注册/处理事件 2345」点名。只读轮，不改运行路径。
"""
import os
import re

import ida_funcs
import ida_hexrays
import ida_name
import idautils
import idc

OUT = r"D:\115us\analysis\dumps\skin-noti"

CTOR = 0x14449CAF0          # 1551 的登记点 + 496 字节单例 ctor
SINGLETON = 0x14E659EA8     # qword_14E659EA8
UI_SINGLETON = 0x14E683C78  # qword_14E683C78
OPCODE = 1551
EVENT_ID = 2345


def fname(ea):
    return ida_name.get_name(ea) or ""


def body_of(ea):
    try:
        return str(ida_hexrays.decompile(ea))
    except Exception:
        return ""


def dump(tag, text):
    name = "df53_%s.c" % re.sub(r"\W+", "_", tag)
    with open(os.path.join(OUT, name), "w", encoding="utf-8") as h:
        h.write("// %s\n\n%s\n" % (tag, text))
    return name


def owners_of(ea):
    out = []
    for xr in idautils.XrefsTo(ea, 0):
        f = ida_funcs.get_func(xr.frm)
        if f and f.start_ea not in out:
            out.append(f.start_ea)
    return out


def size_of(ea):
    f = ida_funcs.get_func(ea)
    return (f.end_ea - f.start_ea) if f else 0


print("=== 1. call-site constants ===", flush=True)
try:
    ctor_callers = owners_of(CTOR)
    print("callers of sub_14449CAF0 (registrar/ctor): %d" % len(ctor_callers), flush=True)
    for start in ctor_callers:
        print("  %s %s size=%d" % (hex(start), fname(start), size_of(start)), flush=True)
    ref_owners = owners_of(SINGLETON)
    print("functions touching qword_14E659EA8: %d" % len(ref_owners), flush=True)
    senders = []
    for start in ref_owners:
        b = body_of(start)
        if not b:
            continue
        if re.search(r"\b%d\b|0x%X\b|0x%x\b" % (OPCODE, OPCODE, OPCODE), b):
            senders.append(start)
            print("  MENTIONS-OPCODE %s %s size=%d" % (hex(start), fname(start), size_of(start)), flush=True)
            print("    " + dump("opcode_ref_%s" % fname(start), b), flush=True)
    for start in ref_owners:
        if start in senders:
            continue
        if size_of(start) > 0x800:
            continue
        b = body_of(start)
        if b:
            print("    " + dump("singleton_user_%s" % fname(start), b), flush=True)
except Exception as e:
    print("SECTION 1 FAILED: %r" % (e,), flush=True)

print("=== 2. store offsets ===", flush=True)
try:
    ref_owners = owners_of(UI_SINGLETON)
    print("functions touching qword_14E683C78: %d" % len(ref_owners), flush=True)
    hits = []
    for start in ref_owners:
        if size_of(start) > 0x1400:
            print("  skip big %s %s size=%d" % (hex(start), fname(start), size_of(start)), flush=True)
            continue
        b = body_of(start)
        if b and re.search(r"\b%d\b|0x%X\b" % (EVENT_ID, EVENT_ID), b):
            hits.append(start)
            print("  MENTIONS-EVENT %s %s size=%d" % (hex(start), fname(start), size_of(start)), flush=True)
            print("    " + dump("event2345_%s" % fname(start), b), flush=True)
    print("event-2345 mentioners: %d" % len(hits), flush=True)
except Exception as e:
    print("SECTION 2 FAILED: %r" % (e,), flush=True)

print("=== 3. parent/child calls ===", flush=True)
try:
    # 事件总线的登记侧：谁把 2345 注册进处理器表（同一批引用者里找「比较/写入 2345」的函数
    # 已经在 §2 覆盖，这里补一条：反编译 sub_146694510 本体，确认参数含义与它查的是哪张表）。
    for nm in ("sub_146694510", "sub_14668C520"):
        ea = idc.get_name_ea_simple(nm)
        if ea == idc.BADADDR:
            print("  missing %s" % nm, flush=True)
            continue
        b = body_of(ea)
        if b:
            print("  wrote " + dump("bus_%s" % nm, b), flush=True)
            print(b[:4000], flush=True)
except Exception as e:
    print("SECTION 3 FAILED: %r" % (e,), flush=True)

print("\n=== df53 done ===", flush=True)

#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""df54: 谁发 CMD 1551（表情快捷键上行）——按「发包构造器 + 机器码里出现立即数 1551」定位，不再猜。

df52/实机已闭环：按一格上行一帧 CMD 1551，体 `u32 表情皮肤 id, u32 0`（10179 / 10176 / 10175 三格
三个值，都是 configs/skin-storage-items.json 里 instant emoticon 的皮肤 id）。
attempt 1/3 已否证：回 CMD 2039（体 01，客户端 `sub_1444E8CC0` 状态非 0 只发界面事件 2345）
实机四格各按一次**无气泡**（服务端 `emote_use_acknowledged` 六条，帧确实发了）。⇒ 气泡不在这条回包上。
另一个独立理由：2039 的处理器一个体字节都不读，**带不了「哪个角色放哪个表情」**，气泡必须点名角色。

本轮只找发送方。已知发送姿势（df39 `sub_1444F0FE0`，1565 的组装方）：
    v6 = sub_146D74000(x); sub_146D746E0(v6, 1565);      // 写 opcode
    v8 = sub_146D74000(y); sub_146D75B10(v8, buf, len);  // 写体
                       sub_146D75AF0(...)                // 发出
⇒ `sub_146D746E0` 是「设 opcode」的那一步，它的每个调用者就是一个上行命令的组装方。
§1 枚举 `sub_146D746E0` 的调用者，**在函数机器码里扫立即数 1551 的小端字节**（`60 06 00 00`），
命中即候选发送方，再反编译；比按名字/按引用者全库反编译快得多，也不会把「函数里出现过这个数字」
当证据——只有真正的 opcode 写入点才会把它当 4 字节立即数编进指令。
§2 同样扫 2039（`eb 07 00 00`）与 2243（`43 08 00 00`），确认客户端自己发不发这两个。
§3 打印候选发送方的调用者（谁按下去走到它），只到上一层。
"""
import os
import re

import ida_bytes
import ida_funcs
import ida_hexrays
import ida_name
import idautils
import idc

OUT = r"D:\115us\analysis\dumps\skin-noti"

OPCODE_WRITER = 0x146D746E0
OPCODES = {1551: "emote hotkey request", 2039: "CMD 2039", 2243: "NOTI 2243"}


def fname(ea):
    return ida_name.get_name(ea) or ""


def body_of(ea):
    try:
        return str(ida_hexrays.decompile(ea))
    except Exception:
        return ""


def dump(tag, text):
    name = "df54_%s.c" % re.sub(r"\W+", "_", tag)
    with open(os.path.join(OUT, name), "w", encoding="utf-8") as h:
        h.write("// %s\n\n%s\n" % (tag, text))
    return name


def callers_of(ea):
    out = []
    for xr in idautils.XrefsTo(ea, 0):
        if idc.print_insn_mnem(xr.frm) != "call":
            continue
        f = ida_funcs.get_func(xr.frm)
        if f and f.start_ea not in out:
            out.append(f.start_ea)
    return out


def find_le(ea, end, needle):
    """在函数体机器码里找小端 4 字节立即数。"""
    hits = []
    cur = ea
    while cur != idc.BADADDR and cur < end:
        b = ida_bytes.get_bytes(cur, 4)
        if b == needle:
            hits.append(cur)
        cur = idc.next_head(cur, end)
    return hits


print("=== 1. call-site constants ===", flush=True)
try:
    composers = callers_of(OPCODE_WRITER)
    print("functions that write an opcode (callers of sub_146D746E0): %d" % len(composers), flush=True)
    found = {}
    for start in composers:
        f = ida_funcs.get_func(start)
        if not f:
            continue
        for op, label in OPCODES.items():
            hits = find_le(f.start_ea, f.end_ea, bytes([op & 0xFF, (op >> 8) & 0xFF, 0, 0]))
            if not hits:
                continue
            found.setdefault(op, []).append(start)
            print("  OPCODE %d (%s) in %s %s size=%d at %s" % (
                op, label, hex(start), fname(start), f.end_ea - f.start_ea,
                ",".join(hex(x) for x in hits[:3])), flush=True)
    for op, starts in found.items():
        for start in starts:
            b = body_of(start)
            if b:
                print("    wrote " + dump("sender_%d_%s" % (op, fname(start)), b), flush=True)
    if 1551 not in found:
        print("  1551 not written by any opcode-writer caller — the emote request uses another send path", flush=True)
except Exception as e:
    print("SECTION 1 FAILED: %r" % (e,), flush=True)

print("=== 2. store offsets ===", flush=True)
try:
    # 兜底：整段 .text 里扫 `60 06 00 00`，但只报告同时调用 sub_146D74000（取写包对象）的函数。
    writer_callers = set(callers_of(0x146D74000))
    print("functions calling sub_146D74000 (packet writer): %d" % len(writer_callers), flush=True)
    extra = []
    for start in sorted(writer_callers - set(c for v in found.values() for c in v)):
        f = ida_funcs.get_func(start)
        if not f or f.end_ea - f.start_ea > 0x2000:
            continue
        if find_le(f.start_ea, f.end_ea, bytes([1551 & 0xFF, 1551 >> 8, 0, 0])):
            extra.append(start)
    print("extra 1551 holders via the writer path: %d" % len(extra), flush=True)
    for start in extra:
        print("  %s %s" % (hex(start), fname(start)), flush=True)
        b = body_of(start)
        if b:
            print("    wrote " + dump("alt_sender_1551_%s" % fname(start), b), flush=True)
except Exception as e:
    print("SECTION 2 FAILED: %r" % (e,), flush=True)

print("=== 3. parent/child calls ===", flush=True)
try:
    for start in found.get(1551, []):
        up = callers_of(start)
        print("callers of %s %s: %d" % (hex(start), fname(start), len(up)), flush=True)
        for u in up[:20]:
            print("  %s %s" % (hex(u), fname(u)), flush=True)
except Exception as e:
    print("SECTION 3 FAILED: %r" % (e,), flush=True)

print("\n=== df54 done ===", flush=True)

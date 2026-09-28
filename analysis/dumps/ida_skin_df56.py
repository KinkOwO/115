#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""df56: 两段独立探针，都带阳性对照（df54 那次 0 命中就是因为没对照，不能当结论）。

背景（已闭环的事实，见 `CLIENT-MECHANICS.md` §21/§22）：
  * 表情快捷键上行 = CMD 1551，体 `u32 表情皮肤 id, u32 0`（实机三格三个值 10179/10176/10175）。
  * 回 CMD 2039（体仅状态字节 01）实机无气泡 ⇒ 否证；2039 的处理器 `sub_1444E8CC0` 一个体字节都不读，
    只 `sub_146694510(qword_14E683C78, 2345, -1, 0, 1)` 发一个界面事件 ⇒ 气泡要么由 2345 的消费者画
    （那它得从别处拿角色与表情），要么根本由另一帧驱动。df53 抓到的 `sub_141FDBA30` 是**发**2345 的一方。

§1 阳性对照下重做「谁发 1551」：`sub_146D746E0` 是发包姿势里写 opcode 的那一步（df39 的 1565 组装方
   `sub_1444F0FE0` 就调它并传 1565）。对它的每个调用者，在函数机器码里同时扫 **2 字节与 4 字节** 小端
   立即数，opcode 取 1565/1592（已知一定命中，作对照）与 1551/2039（目标）。只有对照命中而目标不命中，
   「1551 不走这条路」才成立。
§2 找 2345 的**消费者表**：事件号与处理函数成对出现在数据段里（注册表形状 `{u32 eventId, ptr handler}`），
   所以扫 `.data`/`.rdata` 里「u32 == 事件号 且 相邻 qword 落在代码段」的位置；对照事件号取 730
   （皮肤仓库窗口，§19 已证 `sub_14667BB90(qword_14E683C78, 730, 0)` 拿得到窗口）与 2875（提示条），
   两个对照都必须有命中，否则本方法不成立、结论作废。
"""
import os
import re

import ida_bytes
import ida_funcs
import ida_hexrays
import ida_name
import ida_segment
import idautils
import idc

OUT = r"D:\115us\analysis\dumps\skin-noti"
OPCODE_WRITER = 0x146D746E0
OPCODES = {1565: "control: 应用/收藏请求", 1592: "control: 复制请求", 1551: "TARGET emote request", 2039: "TARGET emote ack"}
EVENTS = {730: "control: 皮肤仓库窗口", 2875: "control: 提示条", 2345: "TARGET emote bubble?"}
CODE_LO, CODE_HI = 0x140000000, 0x14E000000


def fname(ea):
    return ida_name.get_name(ea) or ""


def body_of(ea):
    try:
        return str(ida_hexrays.decompile(ea))
    except Exception:
        return ""


def dump(tag, text):
    name = "df56_%s.c" % re.sub(r"\W+", "_", tag)
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


def scan_bytes(ea, end, needle):
    hits = []
    cur = ea
    step = 1
    while cur != idc.BADADDR and cur < end:
        b = ida_bytes.get_bytes(cur, len(needle))
        if b == needle:
            hits.append(cur)
        cur = idc.next_head(cur, end) if step else cur + 1
    return hits


print("=== 1. call-site constants ===", flush=True)
try:
    composers = callers_of(OPCODE_WRITER)
    print("callers of sub_146D746E0 (opcode writer): %d" % len(composers), flush=True)
    tally = {}
    found = {}
    for start in composers:
        f = ida_funcs.get_func(start)
        if not f:
            continue
        blob = ida_bytes.get_bytes(f.start_ea, min(f.end_ea - f.start_ea, 0x4000)) or b""
        for op, label in OPCODES.items():
            le4 = bytes([op & 0xFF, (op >> 8) & 0xFF, 0, 0])
            le2 = bytes([op & 0xFF, (op >> 8) & 0xFF])
            hit4, hit2 = le4 in blob, le2 in blob
            tally.setdefault(op, [0, 0])
            if hit4:
                tally[op][0] += 1
            if hit2:
                tally[op][1] += 1
            if hit4 or hit2:
                found.setdefault(op, []).append(start)
    for op in OPCODES:
        c4, c2 = tally.get(op, [0, 0])
        print("  opcode %d (%s): 4-byte=%d 2-byte=%d" % (op, OPCODES[op], c4, c2), flush=True)
    for op in (1551, 2039):
        for start in found.get(op, [])[:12]:
            print("  HIT %d in %s %s" % (op, hex(start), fname(start)), flush=True)
            b = body_of(start)
            if b:
                print("    wrote " + dump("sender_%d_%s" % (op, fname(start)), b), flush=True)
except Exception as e:
    print("SECTION 1 FAILED: %r" % (e,), flush=True)

print("=== 2. store offsets ===", flush=True)
try:
    segs = []
    for i in range(ida_segment.get_segm_qty()):
        s = ida_segment.getnseg(i)
        if s and s.name in (".data", ".rdata", ".bss"):
            segs.append((s.name, s.start_ea, s.end_ea))
    print("data segments: %s" % segs, flush=True)
    for ev, label in EVENTS.items():
        hits = []
        for name, lo, hi in segs:
            cur = lo
            while cur + 12 <= hi:
                v = ida_bytes.get_dword(cur)
                if v == ev:
                    nxt = ida_bytes.get_qword(cur + 8)
                    if CODE_LO <= nxt < CODE_HI:
                        hits.append((cur, name, nxt))
                cur += 8
        print("  event %d (%s): %d candidate {id,padding,handler} entries" % (ev, label, len(hits)), flush=True)
        seen = set()
        for addr, seg, handler in hits[:40]:
            if handler in seen:
                continue
            seen.add(handler)
            print("    %s(%s) -> handler %s %s" % (hex(addr), seg, hex(handler), fname(handler)), flush=True)
            b = body_of(handler)
            if b and len(b) < 40000:
                print("      wrote " + dump("event%d_%s" % (ev, fname(handler)), b), flush=True)
except Exception as e:
    print("SECTION 2 FAILED: %r" % (e,), flush=True)

print("=== 3. parent/child calls ===", flush=True)
try:
    for nm in ("sub_146694510", "sub_14667BB90"):
        ea = idc.get_name_ea_simple(nm)
        if ea == idc.BADADDR:
            continue
        b = body_of(ea)
        if b:
            print("  wrote " + dump("bus_%s" % nm, b), flush=True)
            print(b[:3000], flush=True)
except Exception as e:
    print("SECTION 3 FAILED: %r" % (e,), flush=True)

print("\n=== df56 done ===", flush=True)

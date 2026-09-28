#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""df51: 星星勾选态的写入方在哪（df50 §2 因 `idc.FUNCATTR_NEXT` 不存在而整段没跑）。

df50 已闭合三件事（实机 2026-09-28 用户回报「加入收藏已生效，但星星不是点击点亮、再点取消」）：

  1. `sub_146ECFD90(widget)` **不是**纯读勾选位：`return *(u8*)(widget+141) && dword_14DC6B63C == *(u32*)(widget+496);`
     （`df50_getter_Fd90.c`）⇒ 全局 `dword_14DC6B63C` 与 `widget+496` 不相等时，无论 +141 是什么都返回 false。
  2. 行的复选框由 `sub_1441DBF10` 刷新：它算出 v2 后经子控件 vtable+16 写进去，取值来自
     `sub_141FB6530(*(a1+328))` = `*(u8*)(child+140)`（存储态）。
  3. `sub_1441DCE30` 对 9 个行各调一次 vtable+16，参数 = `!(child+140) && sub_146ED0030(...)`。
     ⇒ 星星画成什么样，只由 child+140 决定；+140 的**写入方**至今没点名。

本轮三个段落：§1 邻域里写 `+140` / `+141` 的函数（用 idautils.Functions() 枚举，不再用 idc 的 NEXT 属性）；
§2 `dword_14DC6B63C` 的读者与写者（它决定 getter 会不会恒 false）；§3 行子对象 vtable+16 的实体现在哪个函数、
它的调用者名单。只读轮，不改运行路径。
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

CHILD_GETTER = 0x141FB6530   # *(u8*)(child+140)
WIDGET_GETTER = 0x146ECFD90  # *(u8*)(w+141) && global == *(u32*)(w+496)
GLOBAL_ID = 0x14DC6B63C      # dword_14DC6B63C

RANGES = [
    (0x141FA0000, 0x141FE0000),
    (0x1441B0000, 0x1441F0000),
    (0x1444D0000, 0x1444F8000),
    (0x146EC0000, 0x146ED8000),
]

ALL_FUNCS = list(idautils.Functions())


def in_ranges(ea):
    return any(lo <= ea < hi for lo, hi in RANGES)


def fname(ea):
    return ida_name.get_name(ea) or ""


def body_of(ea):
    try:
        return str(ida_hexrays.decompile(ea))
    except Exception:
        return ""


def dump(tag, text):
    name = "df51_%s.c" % re.sub(r"\W+", "_", tag)
    with open(os.path.join(OUT, name), "w", encoding="utf-8") as h:
        h.write("// %s\n\n%s\n" % (tag, text))
    return name


def neighbours(center, back, fwd):
    out = []
    for ea in ALL_FUNCS:
        if center - back <= ea <= center + fwd:
            out.append(ea)
    return sorted(out)


print("=== 1. call-site constants ===", flush=True)
try:
    for base, off in ((CHILD_GETTER, 140), (WIDGET_GETTER, 141)):
        print("-- writers of +%d near %s" % (off, hex(base)), flush=True)
        hits = 0
        for start in neighbours(base, 0x400, 0x800):
            b = body_of(start)
            if not b:
                continue
            pat = r"\*\(_[A-Z]+ \*\)\(a1 \+ %d\) =" % off
            if re.search(pat, b):
                hits += 1
                print("  WRITE +%d  %s %s" % (off, hex(start), fname(start)), flush=True)
                print("    " + dump("write%d_%s" % (off, fname(start)), b), flush=True)
        if not hits:
            print("  (none in the immediate neighbourhood; listing bodies)", flush=True)
            for start in neighbours(base, 0x200, 0x400):
                b = body_of(start)
                if b and len(b) < 1200:
                    print("  nb %s %s" % (hex(start), fname(start)), flush=True)
                    print("    " + dump("nb_%s" % fname(start), b), flush=True)
except Exception as e:
    print("SECTION 1 FAILED: %r" % (e,), flush=True)

print("=== 2. store offsets ===", flush=True)
try:
    refs = list(idautils.XrefsTo(GLOBAL_ID, 0))
    print("xrefs to dword_14DC6B63C: %d" % len(refs), flush=True)
    gname = fname(GLOBAL_ID)
    owners = {}
    for xr in refs:
        f = ida_funcs.get_func(xr.frm)
        if not f:
            continue
        d = idc.GetDisasm(xr.frm)
        # 写：目标操作数就是该全局（mov cs:dword_..., eax / !cs:dword_... 之类的赋值形态）
        after = d.split(gname, 1)
        is_write = len(after) == 2 and after[1].lstrip().startswith(",")
        rd, wr = owners.get(f.start_ea, (0, 0))
        if is_write:
            owners[f.start_ea] = (rd, wr + 1)
        else:
            owners[f.start_ea] = (rd + 1, wr)
    for start in sorted(owners):
        rd, wr = owners[start]
        print("  %s %s readers=%d writers=%d %s" % (hex(start), fname(start), rd, wr,
                                                    "PANEL" if in_ranges(start) else ""), flush=True)
        if wr:
            b = body_of(start)
            if b:
                print("    " + dump("global_writer_%s" % fname(start), b), flush=True)
    print("  disasm sample:", flush=True)
    for xr in refs[:8]:
        print("    %s  %s" % (hex(xr.frm), idc.GetDisasm(xr.frm)), flush=True)
except Exception as e:
    print("SECTION 2 FAILED: %r" % (e,), flush=True)

print("=== 3. parent/child calls ===", flush=True)
try:
    # 反编译整段太慢，先用纯反汇编扫「byte ptr [reg+8Ch] / +8Dh 的写入指令」（140=0x8C 是存储态，
    # 141=0x8D 是勾选态），命中点再反编译。
    targets = {"+140": [], "+141": []}
    for lo, hi in RANGES:
        ea = idc.find_func_start(lo)
        for fstart in ALL_FUNCS:
            if not (lo <= fstart < hi):
                continue
            cur = fstart
            end = ida_funcs.get_func(fstart).end_ea if ida_funcs.get_func(fstart) else fstart
            while cur != idc.BADADDR and cur < end:
                d = idc.GetDisasm(cur)
                dl = d.lower()
                if dl.startswith("mov") and ("+8ch]" in dl.replace(" ", "") or "+8dh]" in dl.replace(" ", "")):
                    tail = dl.split("]", 1)[0]
                    operands = d.split(None, 1)[1] if " " in d else ""
                    dest = operands.split(",")[0]
                    key = "+140" if "8Ch" in dest else "+141" if "8Dh" in dest else None
                    if key and dest.strip().lower().startswith("byte ptr"):
                        targets[key].append((cur, fstart, d))
                cur = idc.next_head(cur, end)
    for key in ("+140", "+141"):
        print("-- byte stores to %s: %d" % (key, len(targets[key])), flush=True)
        seen = set()
        for ea, fstart, d in targets[key]:
            if fstart in seen:
                continue
            seen.add(fstart)
            print("  %s in %s %s | %s" % (hex(ea), hex(fstart), fname(fstart), d), flush=True)
        for fstart in sorted(seen):
            b = body_of(fstart)
            if b:
                print("    " + dump("store%s_%s" % (key.strip("+"), fname(fstart)), b), flush=True)
except Exception as e:
    print("SECTION 3 FAILED: %r" % (e,), flush=True)

print("\n=== df51 done ===", flush=True)

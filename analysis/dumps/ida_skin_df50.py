#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""df50: 星星图标点击不点亮——点名行复选框状态的写入方。

df41 已闭合：`sub_1441DD510`（唯一调用者 `sub_1441DBB60`）遍历面板 +3280..+3288 的 48 字节节点，
先用 `sub_146ECFD90(*v3)`（行复选框自己的勾选态）当触发条件，再用 `sub_141FB6530(v3[2])`
（= *(u8*)(child+140)，df41 已反编译，是个纯读）取「已存的状态」，两者不一致才经
`sub_1444F0FE0(mgr, 类别, id, flag)` 发 result 2/3。df47 否证了 `sub_1444E9910` 是星星谓词
（它是排序比较器）。⇒ 勾选态的写入方至今没点名，本轮就是找它：

  §1 谁读 `sub_146ECFD90`（全库计数 + 只打印落在皮肤面板区间里的调用点）；
  §2 `sub_146ECFD90` 自己的伪代码（确认它读的是哪个字段），以及同一模块邻域里
     写同一字段的兄弟函数（勾选态 setter 通常就贴在 getter 旁边）；
  §3 那些 setter 的调用者，同样只打印落在皮肤面板/概要区间里的，并 dump 伪代码。

只读轮：不改任何运行路径，不花 C2S 次数。
"""
import os
import re

import idaapi
import ida_bytes
import ida_funcs
import ida_hexrays
import ida_name
import idautils
import idc

OUT = r"D:\115us\analysis\dumps\skin-noti"

GETTER = 0x146ECFD90
STORE_GETTER = 0x141FB6530

# 皮肤仓库窗口 / 概要绘制所在的区间（df40–df49 反复命中的两段）。
RANGES = [
    (0x1441B0000, 0x1441F0000),   # 面板与行填充
    (0x1444D0000, 0x1444F8000),   # 管理器与协议处理器
    (0x141FB0000, 0x141FDA000),   # 行子对象与概要绘制
]


def in_ranges(ea):
    return any(lo <= ea < hi for lo, hi in RANGES)


def fname(ea):
    return ida_name.get_name(ea) or ""


def owner(ea):
    f = idaapi.get_func(ea)
    if not f:
        return None, ""
    return f.start_ea, fname(f.start_ea)


def body_of(ea):
    try:
        return str(ida_hexrays.decompile(ea))
    except Exception:
        return ""


def dump(tag, text):
    name = "df50_%s.c" % re.sub(r"\W+", "_", tag)
    path = os.path.join(OUT, name)
    with open(path, "w", encoding="utf-8") as h:
        h.write("// %s\n\n%s\n" % (tag, text))
    return name


print("=== 1. call-site constants ===")
try:
    refs = list(idautils.XrefsTo(GETTER, 0))
    print("xrefs to sub_146ECFD90 (getter): %d" % len(refs))
    seen = {}
    for xr in refs:
        start, nm = owner(xr.frm)
        if start is None:
            continue
        seen.setdefault(start, (nm, 0))
        seen[start] = (nm, seen[start][1] + 1)
    for start in sorted(seen):
        nm, cnt = seen[start]
        mark = "PANEL" if in_ranges(start) else "     "
        calls = sum(1 for x in idautils.XrefsTo(start, 0)
                    if idc.print_insn_mnem(x.frm) == "call")
        print("%s %s %s callers=%d uses_getter=%d" % (mark, hex(start), nm, calls, cnt))
except Exception as e:
    print("SECTION 1 FAILED: %r" % (e,))

print("=== 2. store offsets ===")
try:
    text = body_of(GETTER)
    print(dump("getter_Fd90", text) if text else "getter decompile empty")
    # 邻域里逐个函数找「写同一字段」的兄弟（勾选态 setter 通常紧邻 getter）。
    lo = GETTER - 0x120
    hi = GETTER + 0x220
    nb = []
    ea = idc.get_func_attr(lo, idc.FUNCATTR_START)
    if ea == idc.BADADDR:
        ea = lo
    cur = ea
    while cur != idc.BADADDR and cur < hi:
        if cur >= lo:
            nb.append(cur)
        cur = idc.get_func_attr(cur, idc.FUNCATTR_NEXT)
    print("neighbour functions: %d" % len(nb))
    writers = []
    for start in nb:
        b = body_of(start)
        if not b:
            continue
        # 写 a1+<off> 且该 off 与 getter 读的 off 相同
        roff = set(re.findall(r"\*\(_[A-Z]+ \*\)\(a1 \+ (\d+)\)", text))
        woff = set(re.findall(r"\*\(_(?:BYTE|QWORD|DWORD|OWORD) \*\)\(a1 \+ (\d+)\)\s*=", b))
        if woff and roff and (woff & roff):
            writers.append(start)
            print("WRITE-SAME-OFFSET %s %s off=%s" % (hex(start), fname(start), sorted(woff & roff)))
            print(dump("setter_%s" % fname(start), b))
    if not writers:
        print("no neighbour writes the getter offset; dumping all neighbour bodies")
        for start in nb:
            b = body_of(start)
            if b:
                print("  neighbour %s %s" % (hex(start), fname(start)))
                print(dump("nb_%s" % fname(start), b))
except Exception as e:
    print("SECTION 2 FAILED: %r" % (e,))

print("=== 3. parent/child calls ===")
try:
    # child+140 的写入方：全库扫「+140 赋值」太宽，先按区间收敛——只反编译皮肤面板/概要区间
    # 里被 `sub_141FB6530` 的调用者直接或间接用到的函数是不够的，所以这里改为：
    # 列出 child getter 的全部调用者，标出落在区间里的，并把它们的伪代码落盘。
    refs = list(idautils.XrefsTo(STORE_GETTER, 0))
    print("xrefs to sub_141FB6530 (child+140 reader): %d" % len(refs))
    starts = []
    for xr in refs:
        start, nm = owner(xr.frm)
        if start is None or start in starts:
            continue
        starts.append(start)
        calls = sum(1 for x in idautils.XrefsTo(start, 0)
                    if idc.print_insn_mnem(x.frm) == "call")
        mark = "PANEL" if in_ranges(start) else "     "
        print("%s %s %s callers=%d" % (mark, hex(start), nm, calls))
    for start in starts:
        if not in_ranges(start):
            continue
        b = body_of(start)
        if b:
            print(dump("reader_%s" % fname(start), b))
except Exception as e:
    print("SECTION 3 FAILED: %r" % (e,))

print("\n=== df50 done ===")

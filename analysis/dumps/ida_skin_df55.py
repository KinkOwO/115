#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""df55: NOTI1545 的页 4 条目有没有「按职业重映射」——决定两步幻化在本构建里是否可表达。

df54 之后新增的静态事实（PVF 直读，非猜测）：
  * 客户端判断「这件消耗品会登记哪个皮肤」的谓词 `sub_1444EC1B0`（`df34_cand_0x1444ec1b0.c`）是
    这么读的：`sub_145ABB480(template)` 取**道具定义**，要求 `*(u32*)(def+2048) == 169`，
    然后 `v6 = *v5` 取 `def+2056` 那个向量里的**第一个也是唯一一个** u32 当皮肤 id。
    ⇒ 皮肤 id 来自静态道具定义表，**没有**任何按实例（item row）覆盖的入口。
  * 本 PVF 里 `[action type] [add skin storage]` 的武器族只有两个模板，参数都是 40000
    （`stackable/10308001/10308358.stk`、`10308800.stk`，`[stackable type] [waste]`、
    `[action target] [character]`、`[action expiration info] [unlimit]`），
    40000 = `skin/weaponskin/baseform.skn`，也正是页 4 面板填充方唯一硬编码的那条注册记录
    （`0x9C40`）。伤害字体/表情那族的参数则是各自的皮肤 id（如 10160911→3、10325568→10179）。

⇒ 用户描述的「复制产出一件消耗品、右键使用才进武器页」要成立，必须能让那个 40000 在客户端展开成
   **被复制那把武器**的模板 id。本轮就问这一件事：NOTI1545 的读取器（`sub_1444EFF40`）与页 4 的
   条目落库路径上，有没有出现 §18.3 那条按职业重映射（`sub_1473A1580(mgr+1328, id)` /
   `sub_1473A1120(mgr+1328, 职业, id)`）。有 ⇒ 两步可表达，按 40000 设计；没有 ⇒ 消耗品带不动武器
   身份，这条要作为服务端不可达的规格差回报，不改一步式实现。
"""
import os
import re

import ida_funcs
import ida_hexrays
import ida_name
import idautils
import idc

OUT = r"D:\115us\analysis\dumps\skin-noti"

READER = 0x1444EFF40      # NOTI1545 body reader (called by sub_1444ED900 with the page selector)
REMAP_A = 0x1473A1580     # sub_1473A1580(mgr+1328, id)
REMAP_B = 0x1473A1120     # sub_1473A1120(mgr+1328, job, id)
INSERTER = 0x1444E8DF0    # the mgr+8 collection-list writer, for shape contrast
STORE = 0x1444EBD90       # ownership test used by NOTI1546 cases


def fname(ea):
    return ida_name.get_name(ea) or ""


def body_of(ea):
    try:
        return str(ida_hexrays.decompile(ea))
    except Exception:
        return ""


def dump(tag, text):
    name = "df55_%s.c" % re.sub(r"\W+", "_", tag)
    with open(os.path.join(OUT, name), "w", encoding="utf-8") as h:
        h.write("// %s\n\n%s\n" % (tag, text))
    return name


def callees(body):
    return sorted(set(re.findall(r"\bsub_([0-9A-Fa-f]{6,})\b", body)))


print("=== 1. call-site constants ===", flush=True)
try:
    for ea, tag in ((READER, "noti1545_reader"), (STORE, "owned_test"), (INSERTER, "fav_writer")):
        b = body_of(ea)
        if not b:
            print("%s %s: decompile empty" % (tag, hex(ea)), flush=True)
            continue
        print("=== %s %s size=%d" % (tag, fname(ea),
                                     (ida_funcs.get_func(ea).end_ea - ida_funcs.get_func(ea).start_ea)
                                     if ida_funcs.get_func(ea) else 0), flush=True)
        print("  wrote " + dump(tag, b), flush=True)
        hits = [c for c in callees(b) if c in ("1473A1580", "1473A1120")]
        print("  job-remap callees present: %s" % (hits or "NONE"), flush=True)
        print("  all callees: %s" % ",".join(hits and [] or callees(b)[:24]), flush=True)
except Exception as e:
    print("SECTION 1 FAILED: %r" % (e,), flush=True)

print("=== 2. store offsets ===", flush=True)
try:
    # 谁调这两个重映射函数：若只有 1565 组装方与 2641 读取器，那页 4 归属表（1545）确实不经重映射。
    for ea in (REMAP_A, REMAP_B):
        owners = []
        for xr in idautils.XrefsTo(ea, 0):
            f = ida_funcs.get_func(xr.frm)
            if f and idc.print_insn_mnem(xr.frm) == "call" and f.start_ea not in owners:
                owners.append(f.start_ea)
        print("%s callers=%d %s" % (fname(ea), len(owners),
                                    [hex(x) + ":" + fname(x) for x in owners[:14]]), flush=True)
except Exception as e:
    print("SECTION 2 FAILED: %r" % (e,), flush=True)

print("=== 3. parent/child calls ===", flush=True)
try:
    # 页 4 归属表的写入者：1545 读取器把条目塞进哪个容器，那个容器的读者是谁。
    b = body_of(READER)
    for m in re.finditer(r"sub_([0-9A-Fa-f]{6,})\(", b):
        ea = idc.get_name_ea_simple("sub_" + m.group(1))
        if ea == idc.BADADDR:
            continue
        f = ida_funcs.get_func(ea)
        size = (f.end_ea - f.start_ea) if f else 0
        if size and size < 0x400:
            cb = body_of(ea)
            if cb:
                print("  wrote " + dump("reader_callee_sub_" + m.group(1), cb), flush=True)
except Exception as e:
    print("SECTION 3 FAILED: %r" % (e,), flush=True)

print("\n=== df55 done ===", flush=True)

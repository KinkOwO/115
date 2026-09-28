#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""df57: 把「谁发哪个 opcode」整张表拉出来——用调用点前的立即数，不再靠机器码扫常量。

为什么要这张表：#32 表情气泡。已知按快捷键上行 = CMD 1551（体 `u32 表情皮肤 id, u32 0`，实机三格
10179/10176/10175），但两个候选回包都被排除：
  * CMD 2039 —— df56 证明 `sub_1444F0ED0` 是它的**上行组装方**，写 `u8 条数 + 条数×25B`，与 NOTI2243
    同构 ⇒ 2039 是「保存四格表情栏」，回帧只发界面事件 2345 刷新表情栏（§23 第 1 条）。
  * NOTI 1551 —— `sub_14449CDB0` 读 `u8 i, u8 flag, u32 value` 写 54 格表，带不了角色与表情（§21 第 3 条）。
而 df56 §1 那 12 个「机器码里出现 1551」的函数，逐个看过去调用 `sub_146D746E0` 时写的都是别的 opcode
（627/615/713/1546/1394/2207/681…）⇒ 1551 是**通过变量/枚举**传进写包器的，扫常量扫不到。

做法：对 `sub_146D746E0`（第二参数 = opcode）的每个调用点，回溯若干条指令找加载到该参数寄存器的
立即数（`mov edx, imm` / `mov r32d, imm` 后接 xchg，或调用点前一条就是 mov），得到「opcode → 组装函数」
全表；然后直接在表里查 1551、2039、以及聊天/气泡相关的 opcode。同时打印每个 opcode 的调用点数，
用来识别「一条帧被多处广播」的情形。
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
# x64 调用约定：第二参数在 edx。
SECOND_ARG = "edx"
BACK_SCAN = 12

TARGETS = (1551, 2039, 2243, 1565, 1592)


def fname(ea):
    return ida_name.get_name(ea) or ""


def body_of(ea):
    try:
        return str(ida_hexrays.decompile(ea))
    except Exception:
        return ""


def dump(tag, text):
    name = "df57_%s.c" % re.sub(r"\W+", "_", tag)
    with open(os.path.join(OUT, name), "w", encoding="utf-8") as h:
        h.write("// %s\n\n%s\n" % (tag, text))
    return name


def imm_for_second_arg(call_ea):
    """回溯调用点前的指令，找加载进 edx（第二参数）的立即数。"""
    cur = call_ea
    for _ in range(BACK_SCAN):
        cur = idc.prev_head(cur, idc.get_func_attr(call_ea, idc.FUNCATTR_START))
        if cur == idc.BADADDR:
            break
        mn = idc.print_insn_mnem(cur)
        op0 = idc.print_operand(cur, 0)
        if mn == "mov" and op0 == SECOND_ARG and idc.get_operand_type(cur, 1) == idc.o_imm:
            return idc.get_operand_value(cur, 1), hex(cur)
        if mn == "mov" and op0 in ("dx", "r16w") and idc.get_operand_type(cur, 1) == idc.o_imm:
            return idc.get_operand_value(cur, 1), hex(cur)
        # 走到函数头就停
        if cur == idc.get_func_attr(call_ea, idc.FUNCATTR_START):
            break
    return None, None


print("=== 1. call-site constants ===", flush=True)
table = {}
try:
    sites = [xr.frm for xr in idautils.XrefsTo(OPCODE_WRITER, 0)
             if idc.print_insn_mnem(xr.frm) == "call"]
    print("call sites of sub_146D746E0: %d" % len(sites), flush=True)
    unknown = 0
    for ea in sites:
        f = ida_funcs.get_func(ea)
        if not f:
            continue
        op, where = imm_for_second_arg(ea)
        if op is None:
            unknown += 1
            continue
        op &= 0xFFFFFFFF
        table.setdefault(op, []).append((f.start_ea, fname(f.start_ea)))
    print("resolved opcodes: %d, unresolved call sites: %d" % (len(table), unknown), flush=True)
    for op in sorted(table):
        owners = sorted({(a, n) for a, n in table[op]})
        if op in TARGETS or len(owners) <= 3:
            print("  opcode %-6d callsites=%d owners=%s" % (op, len(table[op]),
                                                             ",".join("%s:%s" % (hex(a), n) for a, n in owners[:4])), flush=True)
    with open(os.path.join(OUT, "df57_opcode_senders.txt"), "w", encoding="utf-8") as h:
        for op in sorted(table):
            owners = sorted({(a, n) for a, n in table[op]})
            h.write("%d\t%d\t%s\n" % (op, len(owners),
                                      ";".join("%s:%s" % (hex(a), n) for a, n in owners)))
        h.write("# unresolved call sites: %d\n" % unknown)
    print("wrote df57_opcode_senders.txt", flush=True)
except Exception as e:
    print("SECTION 1 FAILED: %r" % (e,), flush=True)

print("=== 2. store offsets ===", flush=True)
try:
    for op in TARGETS:
        owners = sorted({a for a, _ in table.get(op, [])})
        print("opcode %d: %d owners %s" % (op, len(owners), [hex(x) for x in owners[:6]]), flush=True)
        for start in owners[:4]:
            b = body_of(start)
            if b:
                print("  wrote " + dump("composer_%d_%s" % (op, fname(start)), b), flush=True)
except Exception as e:
    print("SECTION 2 FAILED: %r" % (e,), flush=True)

print("=== 3. parent/child calls ===", flush=True)
try:
    # 1551 的组装方一旦点名，它的调用者就是「按快捷键」那条 UI 路径；把它上一层也落盘，
    # 看它在发之前/之后有没有本地画气泡的调用（那就是「服务端只需回 ack」还是「服务端必须广播」的分界）。
    for op in (1551, 2039):
        for start in sorted({a for a, _ in table.get(op, [])}):
            ups = []
            for xr in idautils.XrefsTo(start, 0):
                if idc.print_insn_mnem(xr.frm) != "call":
                    continue
                g = ida_funcs.get_func(xr.frm)
                if g and g.start_ea not in ups:
                    ups.append(g.start_ea)
            print("callers of %s (opcode %d composer): %d -> %s" % (
                fname(start), op, len(ups), ",".join(hex(x) for x in ups[:10])), flush=True)
            for u in ups[:6]:
                b = body_of(u)
                if b and len(b) < 60000:
                    print("  wrote " + dump("caller_%d_%s" % (op, fname(u)), b), flush=True)
except Exception as e:
    print("SECTION 3 FAILED: %r" % (e,), flush=True)

print("\n=== df57 done ===", flush=True)

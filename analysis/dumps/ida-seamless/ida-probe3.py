# -*- coding: utf-8 -*-
"""探针 3：全库扫「对 module+0x508(1288) 的写」，找谁把无缝加载状态(3)打开。

判据纪律（ida-headless-survey 技能）：
  - 「是不是立即数」用 idc.get_operand_type(...) == o_imm，不看打印格式；
  - 位移过滤用**文本**（508h]）是安全的，位移是结构属性；
  - 带一个已知为正的对照组（同一过滤逻辑跑一个已知含该位移的函数），避免 0 命中被误读。
"""
import idc
import idautils
import ida_funcs
import ida_ua

OUT = r"D:\115us\server\work\dfo-lan\runtime\ida-seamless\probe3-out.txt"
DISP_HINTS = ("508h]", "1288")

fh = open(OUT, "w", encoding="utf-8")
total = 0
hits = []
for f_ea in idautils.Functions():
    try:
        f = ida_funcs.get_func(f_ea)
    except Exception:
        f = None
    if f is None:
        continue
    for ea in idautils.FuncItems(f_ea):
        total += 1
        try:
            dis = idc.GetDisasm(ea)
        except Exception:
            continue
        if not dis or not any(h in dis for h in DISP_HINTS):
            continue
        mn = idc.print_insn_mnem(ea)
        if mn not in ("mov", "lea", "and", "or", "cmp", "add", "sub", "inc", "dec", "test", "xor"):
            continue
        imm = None
        try:
            if idc.get_operand_type(ea, 1) == idc.o_imm:
                imm = idc.get_operand_value(ea, 1)
        except Exception:
            pass
        name = idc.get_func_name(ea)
        hits.append((ea, name, dis, imm))

fh.write("扫描指令数 = %d，位移命中 = %d\n\n" % (total, len(hits)))
fh.write("=== 对 +508h 的**写**（mov 且目的含 508h]）===\n")
for ea, name, dis, imm in hits:
    if dis.startswith("mov") and "508h]" in dis and "[" in dis.split(",")[0]:
        fh.write("  %X  imm=%-8s %-60s %s\n" % (ea, imm, dis, name))
fh.write("\n=== 全部命中（含读/比较）===\n")
for ea, name, dis, imm in hits:
    fh.write("  %X  imm=%-8s %-60s %s\n" % (ea, imm, dis, name))
fh.close()
print("probe3 done, instrs=%d hits=%d" % (total, len(hits)))

# -*- coding: utf-8 -*-
"""探针 6：反编译掉落动画选表函数 sub_140657920（四个 .ani 的唯一引用者）。

它同时引用 EndKeeperOfOrder.ctp + Unique/Legendary/Epic/PrimevalDrop_00.ani，
所以这里就是「档位 → 动画」的映射点。看清档位从哪读的，就知道我们缺哪一帧。
"""
import idc
import ida_hexrays
import ida_funcs
import ida_xref

OUT = r"D:\115us\server\work\dfo-lan\runtime\ida-attunement\probe6-out.txt"
FH = open(OUT, "w", encoding="utf-8")

try:
    ida_hexrays.init_hexrays_plugin()
    FH.write("hexrays ok\n")
except Exception as e:  # noqa
    FH.write("hexrays init failed: %s\n" % e)

EA = 0x140657920
FH.write("\n================ %X %s ================\n" % (EA, idc.get_func_name(EA)))
try:
    cf = ida_hexrays.decompile(EA)
    FH.write(str(cf) if cf is not None else "<None>\n")
except Exception as e:  # noqa
    FH.write("<decompile failed: %s>\n" % e)
FH.flush()

# 邻接信息：这个函数前后还有谁（2838 解析器就在 140656A00）
f = ida_funcs.get_func(EA)
FH.write("\nboundary: %X .. %X size=%d\n" % (f.start_ea, f.end_ea, f.end_ea - f.start_ea))

FH.write("\n== 该函数的调用方 ==\n")
n = 0
cur = ida_xref.get_first_cref_to(f.start_ea)
while cur != idc.BADADDR and n < 40:
    f2 = ida_funcs.get_func(cur)
    FH.write("  %016X in %s@%s\n" % (cur, idc.get_func_name(f2.start_ea) if f2 else "-",
                                     ("%X" % f2.start_ea) if f2 else "-"))
    cur = ida_xref.get_next_cref_to(f.start_ea, cur)
    n += 1
FH.write("callers=%d\n" % n)
FH.flush()

# 附近的具名数据/函数，帮助认表
FH.write("\n== 0x140657800..0x140657B00 之间的名字 ==\n")
ea = 0x140657800
while ea < 0x140657B00:
    nm = idc.get_name(ea)
    if nm:
        FH.write("  %016X %s\n" % (ea, nm))
    ea += 1
FH.close()
print("probe6 done")

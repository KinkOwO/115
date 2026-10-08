# -*- coding: utf-8 -*-
"""探针 8：谁读调律模块（sub_1406B1BD0() 返回的单例）的 +80 / +84 / +88 三个档位字段。

这三个字段就是 noti 2859 写进去的东西；找到读它们的函数，
就知道「珠子/动画」是被哪一档驱动的。
"""
import idc
import ida_hexrays
import ida_funcs
import ida_xref

OUT = r"D:\115us\server\work\dfo-lan\runtime\ida-attunement\probe8-out.txt"
FH = open(OUT, "w", encoding="utf-8")

try:
    ida_hexrays.init_hexrays_plugin()
    FH.write("hexrays ok\n")
except Exception as e:  # noqa
    FH.write("hexrays init failed: %s\n" % e)

GETTER = 0x1406B1BD0
FH.write("\n==== 单例 getter %X 的调用方 ====\n" % GETTER)
callers = []
cur = ida_xref.get_first_cref_to(GETTER)
while cur != idc.BADADDR and len(callers) < 200:
    f = ida_funcs.get_func(cur)
    if f:
        callers.append(f.start_ea)
    cur = ida_xref.get_next_cref_to(GETTER, cur)
callers = sorted(set(callers))
FH.write("callers=%d\n" % len(callers))
for c in callers:
    FH.write("  %X %s\n" % (c, idc.get_func_name(c)))
FH.flush()

# 只反编译「体量小」的调用方（大函数成本高），并保留源码
FH.write("\n==== 反编译（只挑小的）====\n")
for c in callers:
    f = ida_funcs.get_func(c)
    size = f.end_ea - f.start_ea
    if size > 0x900:
        FH.write("\n---- skip %X size=%d ----\n" % (c, size))
        continue
    FH.write("\n================ %X %s size=%d ================\n" % (c, idc.get_func_name(c), size))
    try:
        cf = ida_hexrays.decompile(c)
        FH.write(str(cf) if cf is not None else "<None>\n")
    except Exception as e:  # noqa
        FH.write("<decompile failed: %s>\n" % e)
    FH.flush()

FH.close()
print("probe8 done")

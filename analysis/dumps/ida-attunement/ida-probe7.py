# -*- coding: utf-8 -*-
"""探针 7：三个目标
  1. 注册点 0x1406B1765 所在的函数 —— 调律之边界模块的初始化，看它一共注册了哪些包。
  2. sub_1406B18D0 —— noti 2859 BOUNDARY_OF_ATTUNEMENT_REWARD 的处理函数。
  3. sub_14105F8C0 —— cmd 2406 BOUNDARY_OF_ATTUNEMENT_HIDDEN_SELECT 的表项。
"""
import idc
import ida_hexrays
import ida_funcs

OUT = r"D:\115us\server\work\dfo-lan\runtime\ida-attunement\probe7-out.txt"
FH = open(OUT, "w", encoding="utf-8")

try:
    ida_hexrays.init_hexrays_plugin()
    FH.write("hexrays ok\n")
except Exception as e:  # noqa
    FH.write("hexrays init failed: %s\n" % e)

TARGETS = [
    (idc.get_func_attr(0x1406B1765, idc.FUNCATTR_START), "注册点所在函数（调律模块 init）"),
    (0x1406B18D0, "noti 2859 handler"),
    (0x14105F8C0, "cmd 2406 table entry"),
]

for ea, label in TARGETS:
    if not ea or ea == idc.BADADDR:
        FH.write("\n================ %s : 找不到函数起点 ================\n" % label)
        continue
    f = ida_funcs.get_func(ea)
    FH.write("\n================ %X %s  (%s..%s) ================\n" % (
        ea, label, hex(f.start_ea) if f else "?", hex(f.end_ea) if f else "?"))
    try:
        cf = ida_hexrays.decompile(ea)
        FH.write(str(cf) if cf is not None else "<None>\n")
    except Exception as e:  # noqa
        FH.write("<decompile failed: %s>\n" % e)
    FH.write("\n")
    FH.flush()

FH.close()
print("probe7 done")

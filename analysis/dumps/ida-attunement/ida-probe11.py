# -*- coding: utf-8 -*-
"""探针 11：调律模块的三个档位字段（+80/+84/+88）的读取方。

vt[2] = sub_1406B25E0（模块 tick）里调用了两个本模块的小函数：
  sub_1406B19C0(a1)  —— 取值
  sub_1406B1C50(a1)  —— 布尔门（决定是否激活 UI 模块 3842）
外加 sub_1406B1D10 与 sub_1406B2980（模块自身的另两个方法）。
一轮只反编译这四个。
"""
import idc
import ida_hexrays

OUT = r"D:\115us\server\work\dfo-lan\runtime\ida-attunement\probe11-out.txt"
FH = open(OUT, "w", encoding="utf-8")
try:
    ida_hexrays.init_hexrays_plugin()
    FH.write("hexrays ok\n")
except Exception as e:  # noqa
    FH.write("hexrays init failed: %s\n" % e)

for ea, label in [
    (0x1406B19C0, "取值（tick 里传给别的模块）"),
    (0x1406B1C50, "布尔门（决定是否开 UI 模块 3842）"),
    (0x1406B1D10, "模块方法 sub_1406B1D10"),
    (0x1406B2980, "模块方法 sub_1406B2980"),
]:
    FH.write("\n================ %X %s ================\n" % (ea, label))
    try:
        cf = ida_hexrays.decompile(ea)
        FH.write(str(cf) if cf is not None else "<None>\n")
    except Exception as e:  # noqa
        FH.write("<decompile failed: %s>\n" % e)
    FH.write("\n")
    FH.flush()
FH.close()
print("probe11 done")

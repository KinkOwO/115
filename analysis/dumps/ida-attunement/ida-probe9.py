# -*- coding: utf-8 -*-
"""探针 9：反编译 cmd 2406 BOUNDARY_OF_ATTUNEMENT_HIDDEN_SELECT 的构造体 sub_1406B1860。

它由 sub_1406B1520（调律模块 init）用 sub_14599D4D0(tbl, 2406, &functor) 注册，
functor 的 body 就是 sub_1406B1860 ⇒ 看清客户端「选择」时到底发什么字节。
顺带把同模块注册的 2343（FORTIFIED_DISCIPLE_OF_DOOM_HIDDEN_SELECT）也对一下。
"""
import idc
import ida_hexrays
import ida_funcs

OUT = r"D:\115us\server\work\dfo-lan\runtime\ida-attunement\probe9-out.txt"
FH = open(OUT, "w", encoding="utf-8")

try:
    ida_hexrays.init_hexrays_plugin()
    FH.write("hexrays ok\n")
except Exception as e:  # noqa
    FH.write("hexrays init failed: %s\n" % e)

TARGETS = [
    (0x1406B1860, "cmd 2406 构造体（functor body）"),
    (0x1406B1BD0, "模块单例 getter"),
]

for ea, label in TARGETS:
    FH.write("\n================ %X %s ================\n" % (ea, label))
    try:
        cf = ida_hexrays.decompile(ea)
        FH.write(str(cf) if cf is not None else "<None>\n")
    except Exception as e:  # noqa
        FH.write("<decompile failed: %s>\n" % e)
    FH.flush()

# 2406 的两个注册点各自所在函数
for site in (0x1406B17A8, 0x1406BA3D8 if False else 0x1406B17A8):
    f = ida_funcs.get_func(site)
    FH.write("\n注册点 %X 在函数 %s@%X\n" % (site, idc.get_func_name(f.start_ea) if f else "-",
                                              f.start_ea if f else 0))
FH.close()
print("probe9 done")

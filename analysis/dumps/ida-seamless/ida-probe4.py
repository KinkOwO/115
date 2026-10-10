# -*- coding: utf-8 -*-
"""探针 4：反编译模块状态机（找「谁把状态写成 3 = 无缝加载」的触发条件）。
  sub_146D19550 —— CNSelectDungeonModule 状态驱动（146D195FD 写 1、146D1A6FA 写 3）
  sub_146D29700 —— 另一处写 2 / 4 的状态点
"""
import idc
import ida_hexrays

OUT = r"D:\115us\server\work\dfo-lan\runtime\ida-seamless\probe4-out.txt"
TARGETS = [
    (0x146D19550, "CNSelectDungeonModule state driver (writes 1 and 3)"),
    (0x146D29700, "writes state 2 / 4"),
]

fh = open(OUT, "w", encoding="utf-8")
try:
    ida_hexrays.init_hexrays_plugin()
except Exception as e:  # noqa
    fh.write("hexrays init failed: %s\n" % e)

for ea, label in TARGETS:
    fh.write("\n================ %X  %s ================\n" % (ea, label))
    fh.write("func=%s\n" % idc.get_func_name(ea))
    try:
        cf = ida_hexrays.decompile(ea)
        fh.write(str(cf) if cf is not None else "<None>")
    except Exception as e:  # noqa
        fh.write("<decompile failed: %s>\n" % e)
    fh.write("\n")
    fh.flush()
fh.close()
print("probe4 done")

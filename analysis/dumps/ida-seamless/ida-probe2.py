# -*- coding: utf-8 -*-
"""探针 2：反编译两个关键函数（一轮只做一件事，2 个函数）。
  sub_145243AF0 —— 客户端 CMD2062 (DUNGEON_DIRECT_MOVE) 的实现（由 CMDFUNC_ 串 xref 定位）
  sub_146D1AC90 —— CNSelectDungeonModule::Proc_SeamlessLoading
"""
import idc
import ida_hexrays
import idaapi

OUT = r"D:\115us\server\work\dfo-lan\runtime\ida-seamless\probe2-out.txt"
TARGETS = [
    (0x145243AF0, "client CMD2062 DUNGEON_DIRECT_MOVE impl"),
    (0x146D1AC90, "CNSelectDungeonModule::Proc_SeamlessLoading"),
]

fh = open(OUT, "w", encoding="utf-8")
try:
    ida_hexrays.init_hexrays_plugin()
    fh.write("hexrays ok\n")
except Exception as e:  # noqa
    fh.write("hexrays init failed: %s\n" % e)

for ea, label in TARGETS:
    fh.write("\n================ %X  %s ================\n" % (ea, label))
    fh.write("func=%s\n" % idc.get_func_name(ea))
    try:
        cf = ida_hexrays.decompile(ea)
        fh.write(str(cf) if cf is not None else "<decompile returned None>")
    except Exception as e:  # noqa
        fh.write("<decompile failed: %s>\n" % e)
    fh.write("\n")
    fh.flush()

fh.close()
print("probe2 done")

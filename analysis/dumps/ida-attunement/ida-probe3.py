# -*- coding: utf-8 -*-
"""探针 3（一轮只做一件事）：2838 解析器 sub_140656A00 到底把那两个 u32 写到哪、谁读。

目标：
  1. 反编译 sub_140656A00，看清写入 this 的偏移（我们已知是「引子 / 誓约」两个字段）。
  2. 列出它的调用方（确认它就是 noti 2838 的处理函数）。
不反编译别的，控住成本。
"""
import idc
import ida_hexrays
import ida_funcs
import ida_xref

OUT = r"D:\115us\server\work\dfo-lan\runtime\ida-attunement\probe3-out.txt"
FH = open(OUT, "w", encoding="utf-8")

try:
    ida_hexrays.init_hexrays_plugin()
    FH.write("hexrays ok\n")
except Exception as e:  # noqa
    FH.write("hexrays init failed: %s\n" % e)

EA = 0x140656A00
FH.write("\n================ %X %s ================\n" % (EA, idc.get_func_name(EA)))
try:
    cf = ida_hexrays.decompile(EA)
    FH.write(str(cf) if cf is not None else "<None>\n")
except Exception as e:  # noqa
    FH.write("<decompile failed: %s>\n" % e)
FH.flush()

FH.write("\n== 调用方（代码 xref 到入口）==\n")
f = ida_funcs.get_func(EA)
start = f.start_ea if f else EA
seen = []
for xr in ida_xref.xrefs_from_to(0, 0) if False else []:
    pass
ea = start
n = 0
while True:
    nxt = ida_xref.get_first_cref_to(ea)
    if nxt == idc.BADADDR:
        break
    f2 = ida_funcs.get_func(nxt)
    FH.write("  from %X  in %s (%s)\n" % (nxt, idc.get_func_name(f2.start_ea) if f2 else "-",
                                          ("%X" % f2.start_ea) if f2 else "-"))
    ea = nxt
    n += 1
    if n > 40:
        break
FH.write("callers=%d\n" % n)
FH.flush()
FH.close()
print("probe3 done")

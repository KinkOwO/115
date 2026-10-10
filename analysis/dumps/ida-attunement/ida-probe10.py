# -*- coding: utf-8 -*-
"""探针 10：调律模块（noti 2859 写进去的那三个档位字段 +80/+84/+88）的**消费者**。

思路：模块对象是 `sub_146E8BA20(240)` 出来的 240 字节实例，其 vtable 是 `off_1492EAFE0`。
虚拟方法表就是模块的对外接口，逐个反编译小的那个，找谁读 +80/+84/+88（0x50/0x54/0x58）。
"""
import idc
import ida_bytes
import ida_funcs
import ida_hexrays

OUT = r"D:\115us\server\work\dfo-lan\runtime\ida-attunement\probe10-out.txt"
VT = 0x1492EAFE0
FH = open(OUT, "w", encoding="utf-8")

try:
    ida_hexrays.init_hexrays_plugin()
    FH.write("hexrays ok\n")
except Exception as e:  # noqa
    FH.write("hexrays init failed: %s\n" % e)

FH.write("== vtable %X ==" % VT + chr(10))
entries = []
for i in range(24):
    ea = VT + i * 8
    p = ida_bytes.get_qword(ea)
    f = ida_funcs.get_func(p)
    FH.write("  [%2d] %016X -> %016X %s\n" % (i, ea, p, idc.get_func_name(p)))
    if f and p != 0:
        entries.append((i, p, f.start_ea, f.end_ea - f.start_ea))
FH.flush()

FH.write(chr(10) + "== 反编译（体积 < 0x700 的）==" + chr(10))
for i, p, start, size in entries:
    if size > 0x700:
        FH.write("  -- skip vt[%d] %X size=%d" % (i, start, size) + chr(10))
        continue
    FH.write(chr(10) + "---- vt[%d] %X %s size=%d ----" % (i, start, idc.get_func_name(start), size) + chr(10))
    try:
        cf = ida_hexrays.decompile(start)
        FH.write(str(cf) if cf is not None else "<None>")
    except Exception as e:  # noqa
        FH.write("<decompile failed: %s>" % e)
    FH.write(chr(10))
    FH.flush()
FH.close()
print("probe10 done")

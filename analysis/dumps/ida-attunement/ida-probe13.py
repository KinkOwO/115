# -*- coding: utf-8 -*-
"""探针 13：边界之守模块的访问器 —— 谁读那个「档位」值，读的是哪个字段。

线索（探针 12）：sub_140661020 里
    v9  = sub_140657740(module)          // 取一个整数
    v6  = sub_140657770(module)          // 取一个状态
    v10 = sub_140657740(module) - 1
    if (v10 <= 3) { ... a1[21*(v10+9)] ... }   // 四格选择
⇒ sub_140657740 就是「档位」的读取方。看清它读模块的哪个位移，
   就知道驱动演出的到底是 +88（2838 也写）还是 +80/+84（只有 2859 写）。
"""
import idc
import ida_hexrays
import ida_funcs
import ida_xref

OUT = r"D:\115us\server\work\dfo-lan\runtime\ida-attunement\probe13-out.txt"
FH = open(OUT, "w", encoding="utf-8")
try:
    ida_hexrays.init_hexrays_plugin()
    FH.write("hexrays ok\n")
except Exception as e:  # noqa
    FH.write("hexrays init failed: %s\n" % e)

TARGETS = [
    (0x140657740, "档位 getter（被 sub_140661020 用来算四格索引）"),
    (0x140657770, "状态 getter"),
    (0x1406578B0, "邻居小函数 A"),
    (0x1406578F0, "邻居小函数 B"),
    (0x140657910, "邻居小函数 C"),
    (0x14065AF20, "sub_140661020 里另一条分支调用的"),
]
for ea, label in TARGETS:
    FH.write("\n================ %X %s ================\n" % (ea, label))
    try:
        cf = ida_hexrays.decompile(ea)
        FH.write(str(cf) if cf is not None else "<None>\n")
    except Exception as e:  # noqa
        FH.write("<decompile failed: %s>\n" % e)
    FH.flush()

FH.write("\n==== sub_140657740 的调用方 ====\n")
n = 0
cur = ida_xref.get_first_cref_to(0x140657740)
while cur != idc.BADADDR and n < 60:
    f = ida_funcs.get_func(cur)
    FH.write("  %X %s\n" % (cur, idc.get_func_name(f.start_ea) if f else "-"))
    cur = ida_xref.get_next_cref_to(0x140657740, cur)
    n += 1
FH.write("callers=%d\n" % n)
FH.close()
print("probe13 done")

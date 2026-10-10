# -*- coding: utf-8 -*-
"""探针 14：谁读「档位」访问器 sub_140657910（= *(u32*)(module + 88)）。

如果调用方是掉落演出/通告那类函数 ⇒ 驱动就是 +88（2838 也写这个槽 ⇒ 2859 不是必需）；
如果没人读它而演出另有取值路径 ⇒ 驱动可能是 +80/+84（只有 2859 写）。
"""
import idc
import ida_funcs
import ida_hexrays
import ida_xref

OUT = r"D:\115us\server\work\dfo-lan\runtime\ida-attunement\probe14-out.txt"
FH = open(OUT, "w", encoding="utf-8")
try:
    ida_hexrays.init_hexrays_plugin()
except Exception as e:  # noqa
    FH.write("hexrays init failed: %s\n" % e)

TARGET = 0x140657910
FH.write("==== sub_140657910 的调用方 ====\n")
rows = []
cur = ida_xref.get_first_cref_to(TARGET)
while cur != idc.BADADDR and len(rows) < 80:
    f = ida_funcs.get_func(cur)
    rows.append((cur, f.start_ea if f else 0, f.end_ea - f.start_ea if f else 0))
    cur = ida_xref.get_next_cref_to(TARGET, cur)
FH.write("共 %d 处\n" % len(rows))
seen = set()
for site, start, size in rows:
    FH.write("  site=%X in %s size=%d\n" % (site, idc.get_func_name(start) if start else "-", size))
FH.flush()

FH.write("\n==== 调用方函数（体积 < 0x1200 的全反编译）====\n")
done = 0
for site, start, size in rows:
    if start in seen or not start or size > 0x1200:
        continue
    seen.add(start)
    FH.write("\n================ %X %s size=%d ================\n" % (start, idc.get_func_name(start), size))
    try:
        cf = ida_hexrays.decompile(start)
        FH.write(str(cf) if cf is not None else "<None>")
    except Exception as e:  # noqa
        FH.write("<decompile failed: %s>" % e)
    FH.write("\n")
    FH.flush()
    done += 1
    if done >= 10:
        break
FH.close()
print("probe14 done")

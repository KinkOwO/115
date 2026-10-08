# -*- coding: utf-8 -*-
"""探针 12：找「按索引取那四格掉落演出」的代码。

已知事实：
  - 边界之守模块单例 = qword_14E6388B8（sub_140656A00 里 sub_146E8BA20(968) 分配）
  - sub_140657920 把四格演出路径写进 dword 偏移 58 / 80 / 102 / 124（= 字节 232/320/408/496，
    每条 88 字节、路径在条目 +32 字节处），四格文案键写在 dword 156/164/172/180。
  - 四格顺序 = UniqueDrop / LegendaryDrop / EpicDrop / PrimevalDrop。

要回答的是：**索引表达式**（基址是多少、越界怎么兜底）与它读的是哪个字段。
做法：列出引用该单例的函数，反编译，找读上面那些偏移的地方。
"""
import idc
import ida_funcs
import ida_hexrays
import ida_xref

OUT = r"D:\115us\server\work\dfo-lan\runtime\ida-attunement\probe12-out.txt"
SINGLETON = 0x14E6388B8
FH = open(OUT, "w", encoding="utf-8")

try:
    ida_hexrays.init_hexrays_plugin()
    FH.write("hexrays ok\n")
except Exception as e:  # noqa
    FH.write("hexrays init failed: %s\n" % e)

callers = []
cur = ida_xref.get_first_cref_to(SINGLETON)
while cur != idc.BADADDR and len(callers) < 300:
    f = ida_funcs.get_func(cur)
    if f:
        callers.append((f.start_ea, f.end_ea - f.start_ea))
    cur = ida_xref.get_next_cref_to(SINGLETON, cur)
# 数据引用也算（模块对象地址常被存进别处）
cur = ida_xref.get_first_dref_to(SINGLETON)
while cur != idc.BADADDR and len(callers) < 300:
    f = ida_funcs.get_func(cur)
    if f:
        callers.append((f.start_ea, f.end_ea - f.start_ea))
    cur = ida_xref.get_next_dref_to(SINGLETON, cur)

seen = set()
uniq = []
for ea, size in callers:
    if ea in seen:
        continue
    seen.add(ea)
    uniq.append((ea, size))
FH.write("引用单例的函数 %d 个\n" % len(uniq))
for ea, size in sorted(uniq):
    FH.write("  %X  %s  size=%d\n" % (ea, idc.get_func_name(ea), size))
FH.flush()

# 只反编译「读那几个演出偏移」的函数：先用反汇编扫一遍位移，命中才反编译（省时间）
import re
INTERESTING = re.compile(r"(\+ ?(58|80|102|124|156|164|172|180)\))|(232|320|408|496|624|656|688|720)|(0xE8|0x140|0x198|0x1F0)")

FH.write("\n==== 命中演出偏移的函数（反编译全文）====\n")
hits = 0
for ea, size in sorted(uniq):
    if size > 0x2000:
        continue
    try:
        cf = ida_hexrays.decompile(ea)
    except Exception:
        continue
    if cf is None:
        continue
    text = str(cf)
    if not INTERESTING.search(text):
        continue
    hits += 1
    FH.write("\n================ %X %s size=%d ================\n" % (ea, idc.get_func_name(ea), size))
    FH.write(text)
    FH.write("\n")
    FH.flush()
    if hits >= 12:
        FH.write("\n（命中上限 12，停止）\n")
        break
FH.write("\n命中 %d 个\n" % hits)
FH.close()
print("probe12 done")

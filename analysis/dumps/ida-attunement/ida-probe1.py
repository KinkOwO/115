# -*- coding: utf-8 -*-
"""探针 1：先把 noti 分发表里的处理函数指针读出来（一轮只做这一件事）。

opcodes.tsv 给的 table_slot_va 是「该 id 在 noti 派发表里的槽位地址」，
相邻 id 相差 8 字节 ⇒ 槽位里存的应该就是处理函数指针。
这里只做「读指针 + 打印函数名 + 给函数边界」，不反编译，先确认假设成立。
"""
import idc
import ida_bytes
import ida_funcs

OUT = r"D:\115us\server\work\dfo-lan\runtime\ida-attunement\probe1-out.txt"

SLOTS = [
    (2836, 0x14EF38D50, "OMEN_OF_ORDER_PARTY_INFO"),
    (2837, 0x14EF38D58, "ENDKEEPER_OF_ORDER_INFO"),
    (2838, 0x14EF38D60, "ENDKEEPER_OF_ORDER_REWARD"),
    (2839, 0x14EF38D68, "OATH_SYSTEM_INFO"),
    (2842, 0x14EF38D80, "PRIMER_COLLECTION"),
    (2859, 0x14EF38E08, "BOUNDARY_OF_ATTUNEMENT_REWARD"),
    # 对照组：随便挑两个已知名字的，验证槽位读法没读错表
    (27, 0x14EF35EB8, "ENTER_SELECT_DUNGEON?"),
    (29, 0x14EF35EC8, "START_MAP?"),
]

fh = open(OUT, "w", encoding="utf-8")
fh.write("== noti 派发表槽位读取 ==\n")
for op, slot, name in SLOTS:
    raw = ida_bytes.get_qword(slot)
    f = ida_funcs.get_func(raw)
    fh.write("%5d slot=%X -> %016X  func=%s  start=%s end=%s  name=%s\n" % (
        op, slot, raw,
        idc.get_func_name(raw),
        ("%X" % f.start_ea) if f else "-",
        ("%X" % f.end_ea) if f else "-",
        name,
    ))
fh.flush()

# 顺带把表里 2836..2865 整段打出来，看看是不是连续的函数指针数组
fh.write("\n== 2836..2865 连续槽位（验证相邻差 8 字节）==\n")
for op in range(2836, 2866):
    slot = 0x14EF38D50 + (op - 2836) * 8
    raw = ida_bytes.get_qword(slot)
    fh.write("%5d %X -> %016X %s\n" % (op, slot, raw, idc.get_func_name(raw)))
fh.flush()
fh.close()
print("probe1 done")

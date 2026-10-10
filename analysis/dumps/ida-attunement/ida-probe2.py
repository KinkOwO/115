# -*- coding: utf-8 -*-
"""探针 2：先搞清 0x14EF38xxx / 0x14B03xxxx 到底在哪个段里。

探针 1 在 table_slot_va 上读到全 FF ⇒ 槽位地址不在已加载的段里，或者读法不对。
这一轮只做一件事：列出全部段 + 打印目标地址的 flags/原始字节。
"""
import idc
import ida_bytes
import ida_segment

OUT = r"D:\115us\server\work\dfo-lan\runtime\ida-attunement\probe2-out.txt"
FH = open(OUT, "w", encoding="utf-8")

FH.write("== 全部段 ==\n")
segs = []
n = ida_segment.get_segm_qty()
for i in range(n):
    s = ida_segment.getnseg(i)
    if s is None:
        continue
    segs.append((s.start_ea, s.end_ea, ida_segment.get_segm_name(s)))
    FH.write("%016X-%016X %-12s size=%d\n" % (s.start_ea, s.end_ea, ida_segment.get_segm_name(s), s.end_ea - s.start_ea))
FH.flush()

TARGETS = [0x14EF38E08, 0x14EF38D60, 0x14B03DAF0, 0x14B0749F0]

FH.write("\n== 目标地址归属 ==\n")
for ea in TARGETS:
    hit = "-"
    for st, en, nm in segs:
        if st <= ea < en:
            hit = nm
            break
    b = ida_bytes.get_bytes(ea, 16)
    FH.write("%016X seg=%-12s flags=%08X bytes=%s\n" % (
        ea, hit, idc.get_full_flags(ea),
        b.hex() if b else "<None>"))
FH.flush()

FH.write("\n== 镜像基址/输入文件 ==\n")
FH.write("inf_get_min_ea=%X\n" % idc.get_inf_attr(idc.INF_MIN_EA))
FH.write("inf_get_max_ea=%X\n" % idc.get_inf_attr(idc.INF_MAX_EA))
FH.close()
print("probe2 done")

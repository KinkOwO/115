# -*- coding: utf-8 -*-
"""探针 5：谁在引用那四个按档位的掉落动画（Unique/Legendary/Epic/PrimevalDrop_00.ani）。

思路：这四条字符串就是「珠子/掉落动画」的选表依据。找到引用它们的函数，
就能看出档位是从哪里读出来的 —— 那才是「动画↔掉落对齐」的真正判据。
"""
import idc
import ida_funcs
import ida_xref

OUT = r"D:\115us\server\work\dfo-lan\runtime\ida-attunement\probe5-out.txt"
FH = open(OUT, "w", encoding="utf-8")

STRS = [
    (0x1492D7990, "UniqueDrop_00.ani"),
    (0x1492D7A10, "LegendaryDrop_00.ani"),
    (0x1492D7A90, "EpicDrop_00.ani"),
    (0x1492D7B10, "PrimevalDrop_00.ani"),
    (0x14B27D4C0, "[reward boost dungeon]"),
    (0x14B2D7E20, "[dungeon index boundary of attunement]"),
    (0x14B27FE80, "boundary of attunement"),
    (0x1496C5B50, "Endkeeper Of Order"),
    (0x1492D7910, "Contents/2026/EndKeeperOfOrder/Etc/EndKeeperOfOrder.ctp"),
]

for ea, label in STRS:
    FH.write("\n================ %X  %s ================\n" % (ea, label))
    n = 0
    for kind, xr in [(1, ida_xref.get_first_cref_to(ea)), (0, ida_xref.get_first_dref_to(ea))]:
        cur = xr
        while cur != idc.BADADDR:
            f = ida_funcs.get_func(cur)
            FH.write("  [%s] %016X  func=%s@%s\n" % (
                "code" if kind else "data", cur,
                idc.get_func_name(cur),
                ("%X" % f.start_ea) if f else "-"))
            cur = ida_xref.get_next_cref_to(ea, cur) if kind else ida_xref.get_next_dref_to(ea, cur)
            n += 1
            if n > 60:
                break
    FH.write("  xrefs=%d\n" % n)
    FH.flush()
FH.close()
print("probe5 done")

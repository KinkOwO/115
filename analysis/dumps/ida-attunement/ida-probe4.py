# -*- coding: utf-8 -*-
"""探针 4：IDB 里有符号名 —— 直接把「调律/边界/守序/引子/誓约」相关函数名全列出来。

比反编译便宜得多，而且能直接给出处理函数的名字（上一轮 ida-seamless 就是靠符号名定位的）。
"""
import idc
import ida_name
import idautils
import ida_funcs

OUT = r"D:\115us\server\work\dfo-lan\runtime\ida-attunement\probe4-out.txt"
PATS = [
    "attunement", "boundary", "endkeeper", "order", "primer", "oath",
    "abyss", "omen", "verge", "skyof", "reward", "bead", "splendor",
]

FH = open(OUT, "w", encoding="utf-8")
allnames = []
for ea, name in idautils.Names():
    allnames.append((ea, name))
FH.write("总符号数 = %d\n\n" % len(allnames))

lowmap = {}
for ea, name in allnames:
    lowmap.setdefault(name.lower(), []).append((ea, name))

for pat in PATS:
    FH.write("\n########## 含 %r 的符号 ##########\n" % pat)
    hit = 0
    for ea, name in allnames:
        if pat in name.lower():
            f = ida_funcs.get_func(ea)
            FH.write("  %016X  %-70s %s\n" % (ea, name, "FUNC@%X" % f.start_ea if f else ""))
            hit += 1
            if hit > 120:
                FH.write("  ...(截断)\n")
                break
    FH.write("  hits=%d\n" % hit)
    FH.flush()
FH.close()
print("probe4 done")

# -*- coding: utf-8 -*-
"""探针 1（只查引用，不反编译）：无缝加载 / 直进 / keep state 相关字符串的代码引用点。

纪律来源：ida-headless-survey 技能 —— 一轮只做一件事；输出用绝对路径；
带一个已知为正的对照组，避免「0 命中」被误读成「不存在」。
"""
import idc
import idautils
import ida_bytes

OUT = r"D:\115us\server\work\dfo-lan\runtime\ida-seamless\probe1-out.txt"

TARGETS = [
    (0x1499D30A0, "[SEAMLESS LOADING] PREPARE_LEGION_ENTER_DUNGEON start type/delay/passive"),
    (0x14B169000, "CNSelectDungeonModule::Proc_SeamlessLoading"),
    (0x14B169C70, "CNSelectDungeonModule::onEnterModule_SeamlessLoading"),
    (0x14B169DD0, "CNSelectDungeonModule::onExitModule_SeamlessLoading"),
    (0x14B165980, "CNTownModule::onStartSeamlessLoading"),
    (0x14B2852D0, "[direct eplp on clear dungeon]"),
    (0x14B2D6FC0, "is direct move area"),
    (0x14B2D7170, "direct move next phase guarantee delay time"),
    (0x14B27B1E0, "[direct move keep state]"),
    (0x14B27B258, "[keep buff and summons]"),
]
# 对照：行内含函数名的串，必然被 dgn 解析器引用
CONTROL = [
    (0x14A79ADC0, "CMDFUNC_ENUM_CMDPACKET_DUNGEON_DIRECT_MOVE"),
    (0x14A37C938, "[ENTRY] state : %s"),
]

fh = open(OUT, "w", encoding="utf-8")


def dump(addr, label, tag):
    fh.write("%s %X  %s\n" % (tag, addr, label))
    try:
        b = idc.get_bytes(addr, 32)
    except Exception as e:  # noqa
        b = "<get_bytes failed: %s>" % e
    try:
        loaded = ida_bytes.is_loaded(addr)
    except Exception as e:  # noqa
        loaded = "err:%s" % e
    fh.write("   loaded=%s bytes=%r\n" % (loaded, b))
    n = 0
    for x in idautils.XrefsTo(addr, 0):
        fn = idc.get_func_name(x.frm)
        fh.write("   <- %X type=%d code=%s %s | %s\n" % (x.frm, x.type, x.iscode, fn, idc.GetDisasm(x.frm)))
        n += 1
        if n >= 14:
            break
    if n == 0:
        fh.write("   (no xrefs)\n")
    fh.write("\n")


for a, l in CONTROL:
    dump(a, l, "CTRL")
for a, l in TARGETS:
    dump(a, l, "TGT")
fh.close()
print("probe1 done")

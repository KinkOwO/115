# -*- coding: utf-8 -*-
"""探针：在 DFO.exe 里找「包注册」调用点 ```sub_14599D5D0(tbl, id, handler, 0)```。

特征：edx = id ⇒ 机器码 `BA <id_lo> <id_hi> 00 00`（mov edx, imm32）。
找到后，往后一小段里应能看到 `4C 8D 05 .. .. .. ..`(lea r8, ...) 和 `E8 .. .. .. ..`(call)。
用它就能拿到 2859 / 2839 这些我们还没接线的包的处理函数。
"""
import struct
import re

EXE = r"D:\115us\client\DFO.exe"
BASE = 0x140000000
data = open(EXE, "rb").read()

WANT = {
    2859: "noti BOUNDARY_OF_ATTUNEMENT_REWARD",
    2839: "noti OATH_SYSTEM_INFO",
    2842: "noti PRIMER_COLLECTION",
    2837: "noti ENDKEEPER_OF_ORDER_INFO",
    2836: "noti OMEN_OF_ORDER_PARTY_INFO",
    2838: "noti ENDKEEPER_OF_ORDER_REWARD",
    2406: "cmd BOUNDARY_OF_ATTUNEMENT_HIDDEN_SELECT",
    2755: "noti REWARD_BOOST_DUNGEON_TING_INFO",
}

for pid, label in sorted(WANT.items()):
    pat = b"\xBA" + struct.pack("<I", pid)
    print("\n=== %d %s  pattern=%s ===" % (pid, label, pat.hex()))
    start = 0
    hits = []
    while True:
        i = data.find(pat, start)
        if i < 0:
            break
        # 往后看 64 字节，找 call rel32 + lea r8
        win = data[i:i + 64]
        call = re.search(rb"\xE8(....)", win, re.S)
        lea_r8 = re.search(rb"\x4C\x8D\x05(....)", win, re.S)
        # 必须真的是 mov edx,imm（前一条指令边界不可知，所以只靠「窗口里有 call」做粗筛）
        if call:
            hits.append((i, call.start(), call.group(1)))
        start = i + 1
    print("  mov-edx hits with nearby call: %d" % len(hits))
    for i, coff, rel in hits[:8]:
        tgt = BASE + i + coff + 5 + struct.unpack("<i", rel)[0]
        print("    site_va=%X call@+%d -> target %X" % (BASE + i, coff, tgt))

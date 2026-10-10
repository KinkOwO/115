# -*- coding: utf-8 -*-
"""在 DFO.exe 里搜「包名地址」被静态引用的位置。

opcodes.tsv 给了每个包的 string_va（.rdata 里的密文缓冲，运行时原地解密）。
如果客户端有静态的 id->handler 注册表，那里应该能看到 8 字节小端指针 = string_va。
"""
import struct
import sys

EXE = r"D:\115us\client\DFO.exe"
BASE = 0x140000000

TARGETS = {
    0x14B03DAF0: "noti2859 BOUNDARY_OF_ATTUNEMENT_REWARD",
    0x14B0749F0: "cmd2406 BOUNDARY_OF_ATTUNEMENT_HIDDEN_SELECT",
    0x14B03D280: "noti2838 ENDKEEPER_OF_ORDER_REWARD",
    0x14B03D1C0: "noti2836 OMEN_OF_ORDER_PARTY_INFO",
    0x14B03D2E0: "noti2839 OATH_SYSTEM_INFO",
}

data = open(EXE, "rb").read()
print("size", len(data))

for va, label in TARGETS.items():
    pat8 = struct.pack("<Q", va)
    pat4 = struct.pack("<I", va & 0xFFFFFFFF)
    hits8 = []
    start = 0
    while True:
        i = data.find(pat8, start)
        if i < 0:
            break
        hits8.append(i)
        start = i + 1
    print("\n%s va=%X 8byte-hits=%d" % (label, va, len(hits8)))
    for i in hits8[:12]:
        print("   file_off=%X  va=%X" % (i, BASE + i))
    # 4 字节形式只在附近找，避免海量误命中

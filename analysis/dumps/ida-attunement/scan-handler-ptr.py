# -*- coding: utf-8 -*-
"""从注册点机器码里抠出 handler 指针。

注册调用是 ```sub_14599D5D0(tbl, id, handler, 0)```：
  BA <id>            mov edx, id
  4C 8D 05 <rel32>   lea r8, [rip+rel]   ← handler
  E8 <rel32>         call sub_14599D5D0
"""
import struct

EXE = r"D:\115us\client\DFO.exe"
BASE = 0x140000000
data = open(EXE, "rb").read()

SITES = [
    (0x1406B1765, "noti 2859 BOUNDARY_OF_ATTUNEMENT_REWARD"),
    (0x14105FADD, "cmd  2406 BOUNDARY_OF_ATTUNEMENT_HIDDEN_SELECT"),
    (0x14000644F, "noti 2839 OATH_SYSTEM_INFO"),
    (0x140657943, "noti 2838 ENDKEEPER_OF_ORDER_REWARD"),
    (0x140657979, "noti 2836 OMEN_OF_ORDER_PARTY_INFO"),
]

for site, label in SITES:
    off = site - BASE
    b = data[off:off + 24]
    print("\n=== %X %s ===" % (site, label))
    print("   bytes:", b.hex())
    nxt = off + 5
    b2 = data[nxt:nxt + 7]
    if b2[:3] == b"\x4C\x8D\x05":
        rel = struct.unpack("<i", b2[3:7])[0]
        tgt = BASE + nxt + 7 + rel
        print("   lea r8 -> %X" % tgt)
    else:
        print("   后一条不是 lea r8:", b2.hex())

# -*- coding: utf-8 -*-
"""注册调用是 (rcx=tbl, edx=id, r8=handler, r9=0) ⇒ handler 的 lea r8 在 mov edx 之前。
从 mov edx 往回扫最近的 `4C 8D 05 <rel32>`，算出 handler。
自检：2838 必须算出 sub_140656A00（已知）。"""
import struct

EXE = r"D:\115us\client\DFO.exe"
BASE = 0x140000000
data = open(EXE, "rb").read()

def back_lea(site):
    off = site - BASE
    for d in range(5, 40):
        p = off - d
        if p < 0:
            break
        if data[p:p + 3] == b"\x4C\x8D\x05":
            rel = struct.unpack("<i", data[p + 3:p + 7])[0]
            return BASE + p + 7 + rel, d
    return None, None

SITES = [
    (0x140657943, 2838, "ENDKEEPER_OF_ORDER_REWARD  [自检：应为 140656A00]"),
    (0x14065795E, 2837, "ENDKEEPER_OF_ORDER_INFO"),
    (0x140657979, 2836, "OMEN_OF_ORDER_PARTY_INFO"),
    (0x1406B1765, 2859, "BOUNDARY_OF_ATTUNEMENT_REWARD"),
    (0x14105FADD, 2406, "cmd BOUNDARY_OF_ATTUNEMENT_HIDDEN_SELECT"),
    (0x14000644F, 2839, "OATH_SYSTEM_INFO"),
]

for site, pid, label in SITES:
    tgt, d = back_lea(site)
    print("%-52s id=%-5d site=%X  lea_r8@-%s -> %s" % (
        label, pid, site, d, ("%X" % tgt) if tgt else "?"))

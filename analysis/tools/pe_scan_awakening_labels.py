#!/usr/bin/env python3
"""在 client/DFO.exe 里按明文搜索标签字符串，输出其 VA。

用途：调适系统的规则标签（`[max awakening]` / `[need materials]` / `[upgrade result]` …）
既不在 xorstr 表里（那批是加密常量），也不在 `.equ` 之外的脚本里 —— 它们应当是
**明文常量**。拿到 VA 后用 IDA 反查引用者，即可定位客户端的规则解析与**选档逻辑**。

只读，不写任何仓库文件。
  python scan_plaintext.py
"""

import sys

import pefile

NEEDLES = [
    b"[max awakening]",
    b"[need materials]",
    b"[upgrade result]",
    b"[refund materials]",
    b"[grouping index]",
    b"[condition]",
    b"EquipmentAwakening",
    b"EquipmentAwakeningOption",
    b"EquipmentAwakeningOptionSystem",
]

EXE = r"D:\115us\client\DFO.exe"


def main():
    pe = pefile.PE(EXE, fast_load=True)
    base = pe.OPTIONAL_HEADER.ImageBase
    for needle in NEEDLES:
        print("== %s ==" % needle.decode())
        total = 0
        for section in pe.sections:
            data = section.get_data()
            name = section.Name.rstrip(b"\x00").decode("latin1", "replace")
            start = 0
            shown = 0
            while shown < 12:
                idx = data.find(needle, start)
                if idx < 0:
                    break
                start = idx + 1
                total += 1
                va = base + section.VirtualAddress + idx
                tail = data[idx:idx + 48].split(b"\x00")[0]
                print("   0x%X  %-8s %s" % (va, name, tail.decode("latin1", "replace")))
                shown += 1
        print("   total hits: %d" % total)


if __name__ == "__main__":
    sys.exit(main())

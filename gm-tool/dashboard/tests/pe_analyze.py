# -*- coding: utf-8 -*-
"""DFO.exe PE 分析：VA->文件偏移 + 指定函数反汇编（capstone）。"""
import struct
import sys
from capstone import Cs, CS_ARCH_X86, CS_MODE_64

EXE = r"D:\115us\client\DFO.exe"


def load_pe(path):
    with open(path, "rb") as f:
        data = f.read()
    if data[:2] != b"MZ":
        raise SystemExit("not MZ")
    pe_off = struct.unpack_from("<I", data, 0x3C)[0]
    if data[pe_off:pe_off + 4] != b"PE\0\0":
        raise SystemExit("no PE sig")
    coff = pe_off + 4
    machine, nsec = struct.unpack_from("<HH", data, coff)
    opt_off = coff + 20
    magic = struct.unpack_from("<H", data, opt_off)[0]
    img_base = struct.unpack_from("<Q", data, opt_off + 24)[0] if magic == 0x20B else struct.unpack_from("<I", data, opt_off + 28)[0]
    sec_off = opt_off + (struct.unpack_from("<H", data, coff + 16)[0])
    sections = []
    for i in range(nsec):
        off = sec_off + i * 40
        name = data[off:off + 8].rstrip(b"\0").decode("ascii", "replace")
        vsize, vaddr, rsize, roff = struct.unpack_from("<IIII", data, off + 8)
        sections.append((name, vaddr, vsize, roff, rsize))
    return data, img_base, sections


def va_to_off(data, img_base, sections, va):
    rva = va - img_base
    for name, vaddr, vsize, roff, rsize in sections:
        if vaddr <= rva < vaddr + max(vsize, rsize):
            o = roff + (rva - vaddr)
            if o + 16 <= len(data):
                return o
    return None


def disasm_at(data, img_base, sections, va, count=140):
    off = va_to_off(data, img_base, sections, va)
    if off is None:
        print(f"!! cannot map VA {va:#x}")
        return
    md = Cs(CS_ARCH_X86, CS_MODE_64)
    md.detail = False
    code = data[off:off + 512]
    print(f"=== {va:#x} (file off {off:#x}) ===")
    n = 0
    for ins in md.disasm(code, va):
        print(f"  {ins.address:#x}: {ins.mnemonic:10s} {ins.op_str}")
        n += 1
        if n >= count:
            break


def main():
    data, img_base, sections = load_pe(EXE)
    print(f"image base {img_base:#x}, sections: {[(s[0], hex(s[1]), hex(s[3])) for s in sections]}")
    for va in (0x140889b30, 0x140889e50, 0x140889750, 0x140889a1f):
        disasm_at(data, img_base, sections, va)


if __name__ == "__main__":
    main()

#!/usr/bin/env python3
"""Annotated capstone listing for known client addresses, without IDA.

The authoritative .i64 needs a licensed IDA, but any address inside .text is
file backed and can be disassembled here. RIP-relative operands are annotated
against analysis/dumps/xorstr_addr_to_text.json (114k decrypted strings) and
against printable strings in the image, which is usually enough to recognise a
gate/condition function.

Read-only. Requires pefile + capstone.

Usage:
  python pe_disasm_va.py --pe D:/115us/client/DFO.exe --va 0x1406afcc0,0x1406aae00 \
      --out-dir D:/115us/analysis/dumps/va-listings --count 60
"""

import argparse
import json
import re
from pathlib import Path

import pefile
from capstone import CS_ARCH_X86, CS_MODE_64, Cs

RIP_BASE_REG = 41


def load_xorstr(path: Path) -> dict:
    table = {}
    if not path.exists():
        return table
    data = json.loads(path.read_text(encoding="utf-8"))
    items = data.items() if isinstance(data, dict) else []
    for key, value in items:
        try:
            table[int(key, 16) if isinstance(key, str) else int(key)] = value
        except (TypeError, ValueError):
            continue
    return table


def va_at_or_after(va: int, sections) -> bool:
    return any(s.contains_rva(va - s.image_base) for s in sections if hasattr(s, "contains_rva"))


class Image:
    def __init__(self, path: Path):
        self.pe = pefile.PE(str(path), fast_load=True)
        self.base = self.pe.OPTIONAL_HEADER.ImageBase
        self.data = self.pe.__data__

    def read(self, va: int, size: int) -> bytes:
        off = self.pe.get_offset_from_rva(va - self.base)
        if off is None or off < 0 or off + size > len(self.data):
            return b""
        return self.data[off:off + size]

    def printable(self, va: int, size: int = 48) -> str:
        blob = self.read(va, size)
        if not blob:
            return ""
        match = re.match(rb"[\x20-\x7e]{4,}", blob)
        return match.group(0).decode("ascii") if match else ""


def listing(image: Image, xorstr: dict, va: int, count: int) -> list:
    md = Cs(CS_ARCH_X86, CS_MODE_64)
    md.detail = True
    lines = []
    for insn in md.disasm(image.read(va, count * 16), va):
        note = ""
        for operand in insn.operands:
            if operand.type == 3 and operand.mem.base == RIP_BASE_REG:
                target = insn.address + insn.size + operand.mem.disp
                text = xorstr.get(target) or image.printable(target)
                if text:
                    note = "  ; %s" % text.replace("\n", " ")[:70]
        lines.append("%#x  %-8s %-46s%s" % (insn.address, insn.mnemonic, insn.op_str, note))
        if len(lines) >= count:
            break
    return lines


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--pe", required=True)
    parser.add_argument("--va", required=True, help="comma separated addresses, e.g. 0x1406afcc0")
    parser.add_argument("--out-dir", required=True)
    parser.add_argument("--xorstr", default="D:/115us/analysis/dumps/xorstr_addr_to_text.json")
    parser.add_argument("--count", type=int, default=60)
    args = parser.parse_args()

    image = Image(Path(args.pe))
    xorstr = load_xorstr(Path(args.xorstr))
    print("image base=%#x xorstr entries=%d" % (image.base, len(xorstr)))
    out_dir = Path(args.out_dir)
    out_dir.mkdir(parents=True, exist_ok=True)
    summary = []
    for text in args.va.split(","):
        va = int(text.strip(), 16)
        lines = listing(image, xorstr, va, args.count)
        path = out_dir / ("%x.txt" % va)
        path.write_text("\n".join(lines) + "\n", encoding="utf-8")
        summary.append({"va": hex(va), "instructions": len(lines), "file": str(path)})
        print("=== %#x (%d instructions) -> %s" % (va, len(lines), path))
        for line in lines:
            print("  " + line)
    (out_dir / "summary.json").write_text(json.dumps(summary, indent=2), encoding="utf-8")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

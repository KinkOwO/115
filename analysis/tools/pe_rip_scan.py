#!/usr/bin/env python3
"""Locate code that references known handler-table slots and enum strings.

Why: the legion/apocalypse handler table lives in the uninitialised tail of
.data (VirtualSize 0x1760378 > SizeOfRawData 0x9d2000), so the stored function
pointers do not exist on disk. The slot *addresses* and the enum strings are
file backed, so the registration code that writes those slots can be found by
scanning .text for RIP-relative references to them. That code is where the
handler function address shows up.

Read-only. Requires pefile + capstone (both present in the WorkBuddy managed
python and in tools/python; numpy is deliberately NOT required).

Usage:
  python pe_rip_scan.py --pe D:/115us/client/DFO.exe --opcodes D:/115us/analysis/dumps/opcodes.tsv \
      --names LEGION_START,LEGION_OPERATION_SELECT --out D:/115us/analysis/dumps/rip-scan.json
"""

import argparse
import json
from array import array
from pathlib import Path

import pefile
from capstone import CS_ARCH_X86, CS_MODE_64, Cs

RIP_BASE_REG = 41  # X86_REG_RIP in capstone
MIN_TAIL = 4       # instruction bytes after the disp field, shortest form
MAX_TAIL = 12      # longest form worth considering


def read_opcodes(path: Path) -> dict:
    rows = {}
    with path.open(encoding="utf-8") as handle:
        header = handle.readline().rstrip("\n").split("\t")
        for line in handle:
            parts = line.rstrip("\n").split("\t")
            if len(parts) == len(header):
                row = dict(zip(header, parts))
                rows[row["name"]] = row
    return rows


def targets_for(rows: dict, names: list) -> list:
    out = []
    for name in names:
        for prefix in ("ENUM_CMDPACKET_", "ENUM_NOTIPACKET_"):
            row = rows.get(prefix + name)
            if row:
                out.append({
                    "name": name,
                    "enum": row["name"],
                    "family": row["family"],
                    "opcode": int(row["id"]),
                    "string_va": int(row["string_va"], 16),
                    "slot_va": int(row["table_slot_va"], 16),
                })
                break
        else:
            out.append({"name": name, "missing": True})
    return out


def build_index(targets: list, image_base: int) -> dict:
    """Map a candidate displacement+offset sum to the refs that expect it.

    A RIP-relative operand stores disp32 at file offset p and resolves against
    the instruction end, so target = image_base + p + tail + disp, i.e.
    disp + p == target_rva - tail. Every accepted tail yields one expected sum.
    """
    index = {}
    for target in targets:
        if target.get("missing"):
            continue
        for kind, key in (("slot", "slot_va"), ("string", "string_va")):
            rva = target[key] - image_base
            for tail in range(MIN_TAIL, MAX_TAIL + 1):
                index.setdefault(rva - tail, []).append(
                    {"name": target["name"], "kind": kind, "target_va": target[key], "tail": tail})
    return index


def scan(raw: bytes, text_va: int, index: dict) -> list:
    hits = []
    seen = set()
    size = len(raw)
    for phase in range(4):
        view = array("i")
        view.frombytes(raw[phase:phase + 4 * ((size - phase) // 4)])
        p = phase
        for value in view:
            entry = index.get(value + p)
            if entry is not None:
                for item in entry:
                    key = (item["name"], item["kind"], p)
                    if key in seen:
                        continue
                    seen.add(key)
                    hits.append({
                        "name": item["name"],
                        "kind": item["kind"],
                        "target_va": hex(item["target_va"]),
                        "disp": value,
                        "tail": item["tail"],
                        "disp_offset": p,
                        "code_va": hex(text_va + p),
                    })
            p += 4
    return hits


def verify(raw: bytes, text_va: int, hits: list) -> list:
    """Keep only hits where the disp field really ends a RIP-relative insn.

    In x86-64 the disp32 of a RIP-relative operand is the last four bytes of
    the instruction, so the instruction end is always disp_offset + 4. Any
    candidate start is tried, and only an exact end match is accepted.
    """
    md = Cs(CS_ARCH_X86, CS_MODE_64)
    md.detail = True
    kept = []
    for hit in hits:
        want = int(hit["target_va"], 16)
        p = hit["disp_offset"]
        end = text_va + p + 4
        confirmed = None
        for start in range(p - 15, p + 1):
            if start < 0:
                continue
            code = raw[start:start + 16]
            for insn in md.disasm(code, text_va + start):
                if insn.address + insn.size != end:
                    break  # decoding past the candidate end: try another start
                for operand in insn.operands:
                    if operand.type == 3 and operand.mem.base == RIP_BASE_REG:
                        if end + operand.mem.disp == want:
                            confirmed = "%#x  %-8s %s" % (insn.address, insn.mnemonic, insn.op_str)
                if confirmed:
                    break
            if confirmed:
                break
        if confirmed:
            hit["insn"] = confirmed
            kept.append(hit)
    return kept


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--pe", required=True)
    parser.add_argument("--opcodes", required=True)
    parser.add_argument("--names", required=True)
    parser.add_argument("--out", required=True)
    parser.add_argument("--debug", type=int, default=0, help="print the first N raw candidates before verification")
    args = parser.parse_args()

    rows = read_opcodes(Path(args.opcodes))
    targets = targets_for(rows, [n.strip() for n in args.names.split(",") if n.strip()])
    missing = [t["name"] for t in targets if t.get("missing")]
    if missing:
        print("not in opcode dump: %s" % ", ".join(missing))

    pe = pefile.PE(args.pe, fast_load=True)
    base = pe.OPTIONAL_HEADER.ImageBase
    text = next(s for s in pe.sections if s.Name.rstrip(b"\x00") == b".text")
    raw = pe.__data__[text.PointerToRawData:text.PointerToRawData + text.SizeOfRawData]
    text_va = base + text.VirtualAddress
    print("text va=%#x raw_off=%#x bytes=%d image_base=%#x" % (text_va, text.PointerToRawData, len(raw), base))

    index = build_index(targets, base)
    hits = scan(raw, text_va, index)
    print("raw candidate hits: %d" % len(hits))
    if args.debug:
        for hit in hits[:args.debug]:
            print("  cand %-28s %-6s p=%#x disp=%#x tail=%d code=%s"
                  % (hit["name"], hit["kind"], hit["disp_offset"], hit["disp"], hit["tail"], hit["code_va"]))
    kept = verify(raw, text_va, hits)
    print("confirmed RIP references: %d" % len(kept))
    for hit in kept:
        print("  %-30s %-6s %s" % (hit["name"], hit["kind"], hit["insn"]))
    Path(args.out).write_text(json.dumps({"targets": targets, "hits": kept}, indent=2), encoding="utf-8")
    print("wrote %s" % args.out)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

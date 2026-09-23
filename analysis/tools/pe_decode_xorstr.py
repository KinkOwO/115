#!/usr/bin/env python3
"""Decode the client's encrypted enum strings offline.

Recovered from sub_146E8C490 (IDA, 2026-09-23):

  header at the string VA:
    byte 0        unknown flag (unused by the decoder)
    byte 1 & 0xFE key seed part -> v14 = (byte1 & 0xFE) | 0x9A714CA0
    u16 at +2     length code  -> length = a2 * (code ^ (seed8 | seed8 << 7))
  body at +4: rolling-key XOR
    dwords: plain = cipher ^ v14 ; v14 = (plain + 65599 * v14) & 0xffffffff
    tail bytes: plain = cipher ^ (v14 & 0xff) ; v14 = (plain + 263 * v14) & 0xffffffff
  a2 is 2 for the registration initializer sub_140069BB0.

This is used to verify that the enum names in analysis/dumps/opcodes.tsv are
the real decoded values rather than guesses.

Usage:
  python pe_decode_xorstr.py --pe D:/115us/client/DFO.exe \
      --opcodes D:/115us/analysis/dumps/opcodes.tsv --family legion --out D:/115us/analysis/dumps/decoded-names.json
"""

import argparse
import json
import struct
from pathlib import Path

import pefile

KEY_SEED = 0x9A714CA0
MUL_DWORD = 65599
MUL_BYTE = 263


def decode(raw: bytes, multiplier: int = 2) -> str:
    seed8 = raw[1] & 0xFE
    key = (seed8 | KEY_SEED) & 0xFFFFFFFF
    code = struct.unpack_from("<H", raw, 2)[0]
    length = (multiplier * (code ^ (seed8 | (seed8 << 7)))) & 0xFFFFFFFF
    if length == 0 or length > 4096:
        raise ValueError("implausible length %d" % length)
    body = raw[4:4 + length]
    out = bytearray()
    dwords = length // 4
    for i in range(dwords):
        value = struct.unpack_from("<I", body, i * 4)[0] ^ key
        out += struct.pack("<I", value)
        key = (value + MUL_DWORD * key) & 0xFFFFFFFF
    for i in range(length % 4):
        value = body[dwords * 4 + i] ^ (key & 0xFF)
        out.append(value)
        key = (value + MUL_BYTE * key) & 0xFFFFFFFF
    # The plaintext is UTF-16LE: measured 56 decoded bytes == 27 characters + NUL.
    text = out.decode("utf-16-le", "replace")
    return text.split("\x00")[0]


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--pe", required=True)
    parser.add_argument("--opcodes", required=True)
    parser.add_argument("--family", default="", help="optional filter, e.g. legion")
    parser.add_argument("--out", required=True)
    args = parser.parse_args()

    pe = pefile.PE(args.pe, fast_load=True)
    base = pe.OPTIONAL_HEADER.ImageBase
    results = []
    with open(args.opcodes, encoding="utf-8") as handle:
        header = handle.readline().rstrip("\n").split("\t")
        rows = [dict(zip(header, line.rstrip("\n").split("\t"))) for line in handle if line.strip()]
    for row in rows:
        if args.family and args.family.lower() not in row["name"].lower():
            continue
        va = int(row["string_va"], 16)
        offset = pe.get_offset_from_rva(va - base)
        raw = pe.__data__[offset:offset + 256]
        try:
            text = decode(raw, 2)
        except Exception as exc:
            text = "<decode failed: %s>" % exc
        declared = row["name"].replace("ENUM_CMDPACKET_", "").replace("ENUM_NOTIPACKET_", "")
        results.append({
            "family": row["family"],
            "opcode": int(row["id"]),
            "hex": row["hex"],
            "string_va": row["string_va"],
            "table_slot_va": row["table_slot_va"],
            "declared_name": declared,
            "decoded": text,
            "match": text.endswith(declared) or declared.endswith(text) or declared in text,
        })
    ok = sum(1 for r in results if r["match"])
    for r in results:
        print("%-5s %-7s %-34s decoded=%-40r %s" % (r["family"], r["hex"], r["declared_name"],
                                                    r["decoded"], "OK" if r["match"] else "MISMATCH"))
    print("matched %d/%d" % (ok, len(results)))
    Path(args.out).write_text(json.dumps(results, indent=2, ensure_ascii=False), encoding="utf-8")
    print("wrote %s" % args.out)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

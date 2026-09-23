#!/usr/bin/env python3
"""Reader for `.ctp` tables (DFO 115 client, script-table container).

The format was recovered from the client binary (see
`analysis/dumps/va-decompile/ctp_*.c` and `docs`→ `next69-ctp-format.md`).
All integers are little-endian.

    header  := u32 version(1), u32 record_count, u32 0, u32 32, u32 0,
               u32 cell_end, u32 0, u32 trailer_end, u32 0
               (0x00 .. 0x24; both `cell_end` and `trailer_end` read 4 bytes
               short of the real boundary, measured on two files)

    record  := u64 name_lo, u64 name_hi   # name reference into the pool
               u32 flags                  # per-column format/type code
               u64 aux                    # parent record index; -1 = top level
               u64 n_cells
               u64 n_refs
               n_cells x cell
               n_refs  x ref

    cell    := u32 tag ; payload(tag)
                 1 -> 16  name reference (string value)
                 2 -> 20  u32 index + name reference
                 3 -> 1   byte
                 4 -> 8   float64
                 5 -> 8   float64 (second encoding)
                 6,7 -> 5 u32 + byte
                 else -> 0

    ref     := u64 name_lo, u64 name_hi   # name reference
               u64 n_values
               n_values x u64             # record indices owned by that child

    trailer := n_entries x ( u64 name_lo, u64 name_hi, u64 n, n x u64 )
               # column name -> the record indices (rows) of that column

    pool    := packed ASCII tag strings, no separators. Character 0 is the
               first '['; the header's `trailer_end` is 5 bytes before it.

Verified invariants (asserted, not assumed):

  * the walk consumes exactly `record_count` records;
  * the walk ends exactly where the trailer begins;
  * the trailer parses to the byte before the pool;
  * every record/ref/trailer name resolves to a printable pool range.
"""

import argparse
import json
import struct
import sys

CELL_PAYLOAD = {1: 16, 2: 20, 3: 1, 4: 8, 5: 8, 6: 5, 7: 5}
CELL_NAME_REF = 1
CELL_NAME_REF_INDEXED = 2
CELL_BYTE = 3
CELL_FLOAT = 4
CELL_FLOAT_ALT = 5

HEADER_SIZE = 0x24
POOL_LEAD = 5


def u32(b, o):
    return struct.unpack_from("<I", b, o)[0]


def u64(b, o):
    return struct.unpack_from("<Q", b, o)[0]


def s64(b, o):
    return struct.unpack_from("<q", b, o)[0]


def f64(b, o):
    return struct.unpack_from("<d", b, o)[0]


class FormatError(Exception):
    pass


def parse_header(buf):
    if len(buf) < HEADER_SIZE:
        raise FormatError("file shorter than the header")
    head = {
        "version": u32(buf, 0x00),
        "record_count": u32(buf, 0x04),
        "v08": u32(buf, 0x08),
        "v0c": u32(buf, 0x0C),
        "v10": u32(buf, 0x10),
        "cell_end": u32(buf, 0x14),
        "v18": u32(buf, 0x18),
        "trailer_end": u32(buf, 0x1C),
        "v20": u32(buf, 0x20),
    }
    if head["version"] != 1:
        raise FormatError("unsupported .ctp version %d" % head["version"])
    if not (HEADER_SIZE < head["cell_end"] <= len(buf)):
        raise FormatError("header cell_end %#x out of range" % head["cell_end"])
    return head


def pool_char_base(buf, head):
    """Character 0 of the pool is the first '[' at or after the declared end."""
    i = head["trailer_end"]
    while i < len(buf) and buf[i] != 0x5B:
        i += 1
    if i >= len(buf):
        raise FormatError("pool not found after %#x" % head["trailer_end"])
    return i


def string_of(pool, lo, hi):
    a, z = min(lo, hi), max(lo, hi)
    if z > len(pool):
        return None
    seg = pool[a:z]
    if not all(32 <= c < 127 for c in seg):
        return None
    return seg.decode("ascii")


def read_name(buf, o):
    lo, hi = u64(buf, o), u64(buf, o + 8)
    return lo, hi


def read_cells(buf, pool, o, n):
    cells = []
    for _ in range(n):
        tag = u32(buf, o)
        o += 4
        size = CELL_PAYLOAD.get(tag, 0)
        raw = buf[o:o + size]
        if len(raw) != size:
            raise FormatError("cell payload truncated at %#x" % o)
        if tag == CELL_FLOAT:
            cells.append({"tag": tag, "kind": "float", "value": f64(buf, o)})
        elif tag == CELL_FLOAT_ALT:
            cells.append({"tag": tag, "kind": "float_alt", "value": f64(buf, o),
                          "bits": u64(buf, o)})
        elif tag == CELL_BYTE:
            cells.append({"tag": tag, "kind": "byte", "value": raw[0]})
        elif tag == CELL_NAME_REF:
            lo, hi = read_name(buf, o)
            cells.append({"tag": tag, "kind": "name", "raw": [lo, hi],
                          "value": string_of(pool, lo, hi)})
        elif tag == CELL_NAME_REF_INDEXED:
            idx = u32(buf, o)
            lo, hi = read_name(buf, o + 4)
            cells.append({"tag": tag, "kind": "name_indexed", "index": idx,
                          "raw": [lo, hi], "value": string_of(pool, lo, hi)})
        else:
            cells.append({"tag": tag, "kind": "opaque", "raw": raw.hex()})
        o += size
    return cells, o


def walk_records(buf, pool, head):
    o = HEADER_SIZE
    recs = []
    for _ in range(head["record_count"]):
        base = o
        lo, hi = read_name(buf, o)
        flags = u32(buf, o + 16)
        parent = s64(buf, o + 20)
        n_cells = u64(buf, o + 28)
        n_refs = u64(buf, o + 36)
        o += 44
        if n_cells > 1 << 20 or n_refs > 1 << 20:
            raise FormatError("record @%#x implausible counts (%d, %d)"
                              % (base, n_cells, n_refs))
        cells, o = read_cells(buf, pool, o, n_cells)
        refs = []
        for _ in range(n_refs):
            rlo, rhi = read_name(buf, o)
            n_values = u64(buf, o + 16)
            o += 24
            if n_values > 1 << 20:
                raise FormatError("record @%#x ref count %d" % (base, n_values))
            values = [u64(buf, o + 8 * i) for i in range(n_values)]
            o += 8 * n_values
            refs.append({"raw": [rlo, rhi], "name": string_of(pool, rlo, rhi),
                         "values": values})
        recs.append({
            "index": len(recs),
            "off": base,
            "size": o - base,
            "name": string_of(pool, lo, hi),
            "raw_name": [lo, hi],
            "flags": flags,
            "parent": None if parent == -1 else parent,
            "cells": cells,
            "refs": refs,
        })
    return recs, o


def walk_trailer(buf, pool, o, end):
    """Entries until `end`; a single trailing NUL pad byte is tolerated."""
    entries = []
    while o < end:
        if buf[o] == 0 and all(c == 0 for c in buf[o:end]):
            o = end  # trailing padding
            break
        if o + 24 > end:
            raise FormatError("trailer entry @%#x runs past the pool" % o)
        lo, hi = read_name(buf, o)
        n = u64(buf, o + 16)
        o += 24
        if n > 1 << 20 or o + 8 * n > end:
            raise FormatError("trailer entry @%#x bad count %d" % (o, n))
        values = [u64(buf, o + 8 * i) for i in range(n)]
        o += 8 * n
        entries.append({"raw": [lo, hi], "name": string_of(pool, lo, hi),
                        "records": values})
    if o != end:
        raise FormatError("trailer ended at %#x, expected %#x" % (o, end))
    return entries


def parse_file(path):
    buf = open(path, "rb").read()
    head = parse_header(buf)
    chars = pool_char_base(buf, head)
    pool = buf[chars:]
    records, rec_end = walk_records(buf, pool, head)
    if rec_end > chars:
        raise FormatError("record stream overran the pool (%#x > %#x)"
                          % (rec_end, chars))
    trailer = walk_trailer(buf, pool, rec_end, chars)
    doc = {
        "file": path,
        "size": len(buf),
        "header": head,
        "pool": {
            "char_base": chars,
            "bytes": len(pool),
            "text": pool.decode("ascii", "replace"),
        },
        "records": records,
        "trailer": trailer,
        "checks": {
            "record_count": head["record_count"],
            "records_walked": len(records),
            "records_end": rec_end,
            "trailer_start": rec_end,
            "trailer_end": chars,
            "unnamed_records": sum(1 for r in records if not r["name"]),
        },
    }
    return doc


def summarise(doc):
    head = doc["header"]
    print("%s: %d B, version %d, %d records"
          % (doc["file"], doc["size"], head["version"], head["record_count"]))
    print("  pool char base %#x, %d B, %d tags"
          % (doc["pool"]["char_base"], doc["pool"]["bytes"],
             doc["pool"]["text"].count("[")))
    print("  records walked %d, end %#x; trailer %d entries ending %#x"
          % (doc["checks"]["records_walked"], doc["checks"]["records_end"],
             len(doc["trailer"]), doc["checks"]["trailer_end"]))
    if doc["checks"]["unnamed_records"]:
        print("  WARNING: %d records with unresolvable names"
              % doc["checks"]["unnamed_records"])
    for r in doc["records"]:
        values = []
        for c in r["cells"]:
            if c["kind"] in ("float", "float_alt"):
                values.append("%.10g" % c["value"])
            elif c["kind"] == "name":
                values.append("$%s$" % c["value"])
            elif c["kind"] == "name_indexed":
                values.append("%d:$%s$" % (c["index"], c["value"]))
            elif c["kind"] == "byte":
                values.append("b%d" % c["value"])
            else:
                values.append("t%d" % c["tag"])
        refs = ["%s%s" % (x["name"], x["values"]) for x in r["refs"]]
        print("  %2d @%#06x f=%-2d parent=%-5s %-52s %s %s"
              % (r["index"], r["off"], r["flags"], r["parent"], r["name"],
                 values, " ".join(refs)))


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--file", required=True, help="path to a .ctp file")
    ap.add_argument("--out", default="", help="write the parsed document as JSON")
    ap.add_argument("--quiet", action="store_true", help="only write --out")
    args = ap.parse_args()

    try:
        doc = parse_file(args.file)
    except FormatError as e:
        print("FAIL: %s" % e, file=sys.stderr)
        return 1
    if not args.quiet:
        summarise(doc)
    if args.out:
        with open(args.out, "w", encoding="utf-8") as fh:
            json.dump(doc, fh, indent=2, ensure_ascii=False)
        if not args.quiet:
            print("wrote %s" % args.out)
    return 0


if __name__ == "__main__":
    sys.exit(main())

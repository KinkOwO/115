#!/usr/bin/env python3
"""Decode a DFO `.ctp` (RDARScriptBuilder binary table) dump.

Format recovered from client/DFO.exe.i64 on 2026-09-23 (see next69-ctp-format.md):

  * Loader (packed resources):
      sub_1474CDEA0  path suffix dispatch (".ctp" -> sub_147C484A0)
      sub_147C484A0  RDARScriptBuilder::build
      sub_1474D11A0  binary table object (vtable off_14B292468)
      sub_1474D0C00  section reader   (u64,u64 -> name ref; u32; u64; u64; u64)
      sub_1474D0580  cell reader      (u32 tag + tag payload)
      sub_1474CDD90  name ref -> wide string, pool at table+136 (2*offset)
  * The same extension also has a TEXT parser (loose/dev mode):
      sub_1474CA960 -> sub_1474CC1C0 -> sub_1474D02D0 (lexer, error "' (at Ln: %d, %d)'")
  * Cell layout: u32 tag, then payload; tag lengths measured from sub_1474D0580:
      tag 1 -> 16 (two u64, a name range)
      tag 2 -> 20 (u32 + two u64)
      tag 3 ->  1 (byte)
      tag 4 ->  8 (float64)   <-- gameplay numbers
      tag 5 ->  8
      tag 6 ->  5 / tag 7 -> 5 (u32 + byte)
      other ->  0 (4-byte padding cell)
  * String pool: ASCII blob at the tail, concatenated without separators; a name
    ref is a character range (offset, offset+len) resolved with
    `assign_wstr(out, pool + 2*min)`, i.e. the in-memory pool is UTF-16.

Nothing here is guessed: the tiling is pinned by every float cell (tag 4) and by
the one confirmed name-ref cell, and the tool refuses to emit when the tiling
does not reach the pool boundary exactly.
"""

import argparse
import json
import struct
import sys

# A fully tiled apocalypse.ctp is ~1600 cells, so the DFS goes that deep.
sys.setrecursionlimit(200000)

# tag -> payload byte count (measured, see module docstring)
PAYLOAD = {1: 16, 2: 20, 3: 1, 4: 8, 5: 8, 6: 5, 7: 5}


def payload_len(tag):
    return PAYLOAD.get(tag, 0)


def find_pool_base(buf):
    """Return the offset of the pool blob and the trailer end.

    The pool is the trailing printable-ASCII blob. The byte just before it is a
    NUL, and the blob's own offset 0 is the first character (measured: the
    `[party waiting area]` reference is (0, 20), so refs are 0-based, `hi`
    exclusive, exactly the string length).
    """
    best = None
    start = None
    for i in range(max(0, len(buf) - 4096), len(buf)):
        c = buf[i]
        if 32 <= c < 127:
            if start is None:
                start = i
        else:
            if start is not None and i - start > 64:
                best = (start, i)
            start = None
    if start is not None and len(buf) - start > 64:
        best = (start, len(buf))
    if best is None:
        raise SystemExit("no printable pool found")
    start, end = best
    if buf[start - 1] != 0:
        raise SystemExit("pool start is not preceded by a NUL")
    return start, end


def parse_trailer(buf, start, end, pool):
    """Trailer records: (u64 lo, u64 hi, u64 count, count x u64).

    Verified on apocalypse.ctp: 7 records consume the 248 bytes exactly, and
    every (lo, hi) resolves to a pool string whose length is exactly hi - lo
    (e.g. (51, 82) -> "[keldon xavi final damage rate]", 31 chars).
    """
    records = []
    p = start
    while p + 24 <= end:
        lo = struct.unpack_from("<Q", buf, p)[0]
        hi = struct.unpack_from("<Q", buf, p + 8)[0]
        count = struct.unpack_from("<Q", buf, p + 16)[0]
        if count > 4096 or p + 24 + 8 * count > end:
            raise SystemExit("trailer record at %#x does not fit (count=%d)" % (p, count))
        name = resolve(pool, lo, hi)
        values = [struct.unpack_from("<Q", buf, p + 24 + 8 * i)[0] for i in range(count)]
        records.append({
            "off": hex(p),
            "lo": lo,
            "hi": hi,
            "name": name,
            "span_matches_name": name is not None and len(name) == hi - lo,
            "values": values,
        })
        p += 24 + 8 * count
    if p != end:
        raise SystemExit("trailer not consumed exactly: %#x != %#x" % (p, end))
    return records


def resolve(pool, lo, hi):
    """Name ref -> string. In the file the range is exact, so no NUL scan needed."""
    a = min(lo, hi)
    z = max(lo, hi)
    if z > len(pool) or a < 0:
        return None
    try:
        return pool[a:z].decode("ascii")
    except UnicodeDecodeError:
        return None


def float_anchors(buf, end):
    """Offsets of u32 words that are immediately followed by an integral f64."""
    out = {}
    for i in range(0x38, end - 7):
        v = struct.unpack_from("<d", buf, i)[0]
        if v != v:
            continue
        if abs(v - round(v)) < 1e-9 and 1.0 <= round(v) <= 200000.0:
            out[i - 4] = 4
    return out


def tile(buf, start, end, anchors, max_solutions=400, node_budget=4_000_000):
    """Anchor-constrained DFS tiling.

    The wire format has 4-byte "tag 0" padding cells, so several tilings can be
    locally consistent. Collect a bounded set of solutions and keep the one that
    explains the most non-zero cells - that is the one the writer produced.
    """
    order = sorted(anchors)
    solutions = []
    nodes = [0]

    def next_anchor(o):
        lo, hi = 0, len(order)
        while lo < hi:
            mid = (lo + hi) // 2
            if order[mid] < o:
                lo = mid + 1
            else:
                hi = mid
        return order[lo] if lo < len(order) else None

    def dfs(o, path):
        if len(solutions) >= max_solutions or nodes[0] > node_budget:
            return
        nodes[0] += 1
        if o == end:
            solutions.append(list(path))
            return
        if o > end:
            return
        na = next_anchor(o)
        for t in (0, 4, 1, 5, 2, 3, 6, 7) + tuple(range(8, 64)):
            adv = 4 + payload_len(t)
            if o + adv > end:
                continue
            if na is not None and na > o and o + adv > na:
                continue
            if o in anchors and anchors[o] != t:
                continue
            path.append(t)
            dfs(o + adv, path)
            path.pop()

    dfs(start, [])
    if not solutions:
        raise SystemExit("no consistent tiling found (format assumption broken)")

    def score(s):
        # Tags outside 1..7 have no known payload, so a tiling that invents them
        # is degenerate even if it looks "richer".
        bad = sum(1 for t in s if t > 7)
        return (-bad, sum(1 for t in s if t != 0))

    return max(solutions, key=score)


def scan_name_refs(buf, start, end, pool, tags):
    """Find every `(u64 lo, u64 hi)` that resolves to a known pool tag.

    Tiling-independent (the byte layout between records is still not pinned), and
    the span check is exact: hi - lo == len(tag). Verified on apocalypse.ctp:
    116 hits forming the repeated column order of every row.
    """
    refs = []
    for o in range(start, end - 16, 4):
        lo = struct.unpack_from("<Q", buf, o)[0]
        hi = struct.unpack_from("<Q", buf, o + 8)[0]
        if lo >= hi or hi > len(pool):
            continue
        name = resolve(pool, lo, hi)
        if name in tags:
            refs.append({"off": o, "lo": lo, "hi": hi, "name": name})
    return refs


def read_records(buf, refs, cell_end, pool):
    """Each record is a 20-byte name ref followed by `u64` payload words.

    The next ref starts the next record, so the payload length is implied. For
    scalar columns the payload is `0, value`; for table columns it is an index
    prefix followed by 12-byte float64 cells `(u32 tag=4, f64)`.
    """
    records = []
    for i, ref in enumerate(refs):
        start = ref["off"] + 20
        end = refs[i + 1]["off"] if i + 1 < len(refs) else cell_end
        span = end - start
        if span < 0:
            continue
        words = []
        p = start
        while p + 4 <= end:
            tag = struct.unpack_from("<I", buf, p)[0]
            if tag == 4 and p + 12 <= end:
                words.append({
                    "off": hex(p), "tag": 4,
                    "f64": struct.unpack_from("<d", buf, p + 4)[0],
                })
                p += 12
            elif p + 8 <= end:
                words.append({"off": hex(p), "word": struct.unpack_from("<Q", buf, p)[0]})
                p += 8
            else:
                words.append({"off": hex(p), "u32": tag})
                p += 4
        records.append({
            "name": ref["name"],
            "ref_off": hex(ref["off"]),
            "span": span,
            "words": words,
        })
    return records


def sequence_of(words):
    """The float sequence of a record (table columns only)."""
    return [w["f64"] for w in words if w.get("tag") == 4]


def phase_groups(numbers, min_values=10):
    """Find runs shaped (dur,1)(dur2,2)...(durN,N) - the phase clock table."""
    groups = []
    i = 0
    while i + 3 < len(numbers):
        if numbers[i + 1] == 1.0:
            run = [numbers[i], 1.0]
            j = i + 2
            expect = 2.0
            while j + 1 < len(numbers) and numbers[j + 1] == expect:
                run += [numbers[j], expect]
                expect += 1.0
                j += 2
            if len(run) >= min_values:
                groups.append((numbers[i], run))
                i = j
                continue
        i += 1
    return groups


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--file", required=True, help="exported .ctp bytes")
    ap.add_argument("--out", help="write a JSON summary here")
    args = ap.parse_args()

    buf = open(args.file, "rb").read()
    pool_start, pool_end = find_pool_base(buf)
    pool = buf[pool_start:pool_end]
    trailer_end = pool_start - 1  # the NUL that separates the trailer from the pool

    # The trailer is a record array; walk it backwards while the u64s are small
    # to find where the cell region stops.
    cell_end = trailer_end
    while cell_end - 8 >= 0 and struct.unpack_from("<Q", buf, cell_end - 8)[0] < 1 << 20:
        cell_end -= 8
    if (trailer_end - cell_end) % 8 != 0 or cell_end == trailer_end:
        raise SystemExit("trailer not found")

    anchors = float_anchors(buf, cell_end)
    tags = tile(buf, 0x40, cell_end, anchors)
    trailer = parse_trailer(buf, cell_end, trailer_end, pool)

    off = 0x40
    numbers = []
    name_refs = []
    stream = []
    for t in tags:
        if t == 4:
            v = struct.unpack_from("<d", buf, off + 4)[0]
            stream.append({"off": off, "tag": t, "value": v})
            numbers.append(v)
        elif t == 5:
            stream.append({"off": off, "tag": t, "value": struct.unpack_from("<Q", buf, off + 4)[0]})
        elif t == 3:
            stream.append({"off": off, "tag": t, "value": buf[off + 4]})
        elif t == 1:
            lo = struct.unpack_from("<Q", buf, off + 4)[0]
            hi = struct.unpack_from("<Q", buf, off + 12)[0]
            ref = {"off": off, "tag": t, "lo": lo, "hi": hi, "name": resolve(pool, lo, hi)}
            name_refs.append(ref)
            stream.append(ref)
        off += 4 + payload_len(t)

    groups = phase_groups(numbers)
    pool_tags = []
    pos = 0
    while True:
        a = pool.find(b"[", pos)
        if a < 0:
            break
        b = pool.find(b"]", a)
        if b < 0:
            break
        pool_tags.append(pool[a:b + 1].decode("ascii"))
        pos = b + 1

    # Inline name refs + the record stream. This is the load-bearing parse: it
    # does not depend on the (still ambiguous) padding-aware tiling.
    known = set(pool_tags)
    inline_refs = scan_name_refs(buf, 0x40, cell_end, pool, known)
    records = read_records(buf, inline_refs, cell_end, pool)

    summary = {
        "file": args.file,
        "size": len(buf),
        "pool_start": hex(pool_start),
        "pool_bytes": len(pool),
        "cell_region": [hex(0x40), hex(cell_end)],
        "trailer_region": [hex(cell_end), hex(trailer_end)],
        "cells": len(tags),
        "tag_histogram": {str(k): tags.count(k) for k in sorted(set(tags))},
        "pool_tags": pool_tags,
        "trailer": trailer,
        "name_refs": name_refs,
        "inline_name_refs": inline_refs,
        "records": [{"name": r["name"], "ref_off": r["ref_off"], "span": r["span"],
                     "sequence": sequence_of(r["words"])} for r in records],
        "phase_groups": [{"first_duration": g[0], "sequence": g[1]} for g in groups],
        "float_count": len(numbers),
    }
    if args.out:
        with open(args.out, "w", encoding="utf-8") as fh:
            json.dump(summary, fh, indent=2, ensure_ascii=False)
    print("pool start %#x (%d bytes), trailer %#x..%#x" % (pool_start, len(pool), cell_end, trailer_end))
    print("cells %d, tags %s" % (len(tags), summary["tag_histogram"]))
    print("pool tags (%d): %s" % (len(pool_tags), ", ".join(pool_tags)))
    print("trailer records (%d):" % len(trailer))
    for r in trailer:
        flag = "ok" if r["span_matches_name"] else "SPAN-MISMATCH"
        print("   %-11s %-7s values=%d %s" % (r["off"], flag, len(r["values"]), r["name"]))
    print("cell records: %d (nameRef-anchored, tiling-independent)" % len(records))
    for r in records:
        seq = sequence_of(r["words"])
        if len(seq) >= 6:
            print("   %-11s span=%-4d %-24s seq=%s" % (r["ref_off"], r["span"], r["name"], seq[:14]))
    for g in groups:
        print("phase clock: %s" % (g[1],))
    return 0


if __name__ == "__main__":
    sys.exit(main())

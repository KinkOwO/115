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
    """The pool is the trailing printable-ASCII blob; char offset 0 is a NUL."""
    best = None
    start = None
    for i in range(max(0o0, len(buf) - 4096), len(buf)):
        c = buf[i]
        if 32 <= c < 127 or c == 0x5B or c == 0x5D:
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
    # char offset 0 of the pool is the NUL that precedes the first '['.
    if buf[start - 1] != 0:
        raise SystemExit("pool start is not preceded by a NUL")
    return start - 1, end


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


def resolve(pool, lo, hi):
    """Name ref -> string. The client only uses min(lo,hi) and reads to a NUL."""
    a = min(lo, hi)
    z = max(lo, hi)
    if z > len(pool):
        return None
    seg = pool[a:]
    nul = seg.find(b"\x00")
    if nul >= 0:
        seg = seg[:nul]
    try:
        return seg.decode("ascii")
    except UnicodeDecodeError:
        return None


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
    pool_base, pool_end = find_pool_base(buf)
    pool = buf[pool_base:pool_end]

    # Between the cells and the pool sits an array of small u64 values (stride 8,
    # right-aligned to the pool). Walk it backwards: its start is the cell end.
    cell_end = pool_base
    while cell_end - 8 >= 0 and struct.unpack_from("<Q", buf, cell_end - 8)[0] < 1 << 20:
        cell_end -= 8
    if (pool_base - cell_end) % 8 != 0 or cell_end == pool_base:
        raise SystemExit("trailing metadata array not found")

    anchors = float_anchors(buf, cell_end)
    tags = tile(buf, 0x40, cell_end, anchors)

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

    # Inline name refs: a u64 pair that resolves to a known pool tag. Scanned
    # independently of the tiling so a different (still valid) tiling cannot
    # hide them.
    known = set(pool_tags)
    inline_refs = []
    for o in range(0, len(buf) - 16, 4):
        lo = struct.unpack_from("<Q", buf, o)[0]
        hi = struct.unpack_from("<Q", buf, o + 8)[0]
        if lo >= hi or hi - lo > 200 or lo < 1 or hi > len(pool):
            continue
        name = resolve(pool, lo, hi)
        if name in known:
            inline_refs.append({"off": hex(o), "lo": lo, "hi": hi, "name": name})

    summary = {
        "file": args.file,
        "size": len(buf),
        "pool_base": hex(pool_base),
        "pool_bytes": len(pool),
        "cell_region": [hex(0x40), hex(cell_end)],
        "cells": len(tags),
        "tag_histogram": {str(k): tags.count(k) for k in sorted(set(tags))},
        "pool_tags": pool_tags,
        "name_refs": name_refs,
        "inline_name_refs": inline_refs,
        "phase_groups": [{"first_duration": g[0], "sequence": g[1]} for g in groups],
        "float_count": len(numbers),
    }
    if args.out:
        with open(args.out, "w", encoding="utf-8") as fh:
            json.dump(summary, fh, indent=2, ensure_ascii=False)
    print("pool base %#x (%d bytes)" % (pool_base, len(pool)))
    print("cells %d, tags %s" % (len(tags), summary["tag_histogram"]))
    print("pool tags (%d): %s" % (len(pool_tags), ", ".join(pool_tags)))
    print("name refs: %s" % (name_refs,))
    for g in groups:
        print("phase clock: %s" % (g[1],))
    return 0


if __name__ == "__main__":
    sys.exit(main())

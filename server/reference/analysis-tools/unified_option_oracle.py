#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""CMD2377 SET_UNIFIED_OPTION oracle: verify the skill lock merge on real frames.

Why this exists
---------------
An outside report claims this client stores locked skills inside CMD2377
(subtype 0x13) as two compact, ascending 64 slot pages, sends only the slots
that changed since its last baseline, uses slot value 0 as a delete signal, and
expects the server to push the set back in NOTI2827. The frame layout is
confirmed by the frames this gateway already captured; the merge rules are what
this script checks before anyone trusts them.

Frame layout (verified against every captured frame)
----------------------------------------------------
  +00 u32 stamp                 (varies; 0, 1, 0xFFFFFFFF all occur)
  +04 u32                       (the report's "always 1" is wrong)
  +08 5B  FE FF FF FF FF        marker
  +13 u8  scope                 (0 for the skill lock, 1 for the 0x01 blocks)
  +14 u8  subtype               (0x13 skill lock, 0x05 settings, 0x01 other)
  +15 u8  count
  +16 3B  zero
  +19     count x (u16 position, u16 value)
  tail    0..5 zero bytes

Usage
-----
  python unified_option_oracle.py <events.jsonl | run directory | glob>
  python unified_option_oracle.py runtime/roles_*_next37/events.jsonl --expect 9,517

The script prints every frame it finds, merges each subtype 0x13 frame onto the
lock set, and with --expect compares the resulting set against the skills the
player actually locked in game. If the sets differ, the merge rules (or the
report) do not hold for this capture and no server side change should be based
on them yet.
"""

import argparse
import glob
import json
import os
import sys

MARKER = bytes([0xFE, 0xFF, 0xFF, 0xFF, 0xFF])
MARKER_AT = 8
HEADER = 19
EMPTY = 0xFFFF
PAGE_SIZE = 64
PAGE_SLOTS = 128
PAGE_STRIDE = 512
SKILL_LOCK = 0x13


def parse_frame(p):
    """Return (scope, subtype, [(position, value), ...], tail) or raise."""
    if len(p) < HEADER:
        raise ValueError("short frame: %d bytes" % len(p))
    if p[MARKER_AT:MARKER_AT + len(MARKER)] != MARKER:
        raise ValueError("marker mismatch")
    scope, subtype = p[13], p[14]
    count = p[15]
    end = HEADER + count * 4
    if len(p) < end or len(p) > end + 8:
        raise ValueError("count %d does not fit %d bytes" % (count, len(p)))
    if any(p[end:]):
        raise ValueError("non zero padding after the entries")
    entries = []
    for i in range(count):
        off = HEADER + i * 4
        entries.append((p[off] | (p[off + 1] << 8), p[off + 2] | (p[off + 3] << 8)))
    return scope, subtype, entries, len(p) - end


def compact(ids):
    """Rebuild the 128 slot block from a skill id set (two compact pages)."""
    slots = [EMPTY] * PAGE_SLOTS
    page0 = sorted(i for i in ids if i < PAGE_STRIDE)
    page1 = sorted(i - PAGE_STRIDE for i in ids if PAGE_STRIDE <= i < 2 * PAGE_STRIDE)
    for index, value in enumerate(page0[:PAGE_SIZE]):
        slots[index] = value
    for index, value in enumerate(page1[:PAGE_SIZE]):
        slots[PAGE_SIZE + index] = value
    return slots


def collect(slots):
    """Inverse of compact(); slot value 0 never becomes a skill."""
    out = set()
    for pos, value in enumerate(slots):
        if value in (0, EMPTY):
            continue
        out.add(value + (PAGE_STRIDE if pos >= PAGE_SIZE else 0))
    return out


def merge(current, entries):
    """Apply one incremental frame onto the stored set."""
    slots = compact(current)
    if entries:
        first = entries[0][0]
        if first == 0:
            slots[0:PAGE_SIZE] = [EMPTY] * PAGE_SIZE
        elif first == PAGE_SIZE:
            slots[PAGE_SIZE:PAGE_SLOTS] = [EMPTY] * PAGE_SIZE
    for pos, value in entries:
        if pos >= PAGE_SLOTS:
            continue
        slots[pos] = EMPTY if value == 0 else value
    return collect(slots)


def iter_frames(paths):
    """Yield (source, time, plaintext) for every captured CMD2377 frame."""
    files = []
    for path in paths:
        if os.path.isdir(path):
            files.extend(glob.glob(os.path.join(path, "**", "events.jsonl"), recursive=True))
        else:
            files.extend(glob.glob(path))
    for path in sorted(set(files)):
        for line in open(path, encoding="utf-8", errors="replace"):
            if '"id":2377' not in line:
                continue
            try:
                record = json.loads(line)
            except ValueError:
                continue
            if record.get("kind") == "client_frame" and record.get("id") == 2377:
                if record.get("plain_hex"):
                    yield path, record.get("time"), bytes.fromhex(record["plain_hex"])


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("capture", nargs="+", help="events.jsonl, a run directory or a glob")
    parser.add_argument("--expect", default="", help="comma separated skill ids the player actually locked")
    args = parser.parse_args(argv)

    locks = set()
    seen = 0
    for source, when, payload in iter_frames(args.capture):
        try:
            scope, subtype, entries, tail = parse_frame(payload)
        except ValueError as exc:
            print("%s  %s  INVALID: %s" % (when, os.path.basename(os.path.dirname(source)), exc))
            continue
        seen += 1
        head = "len=%3d scope=%d subtype=0x%02x count=%d tail=%d" % (len(payload), scope, subtype, len(entries), tail)
        if subtype == SKILL_LOCK:
            locks = merge(locks, entries)
            print("%s  %s  entries=%s -> locks=%s" % (when, head, entries, sorted(locks)))
        else:
            print("%s  %s  (not a skill lock)" % (when, head))

    print()
    print("frames=%d  final skill set=%s" % (seen, sorted(locks)))
    if args.expect:
        want = {int(v) for v in args.expect.replace(" ", "").split(",") if v}
        if want == locks:
            print("MATCH: the merge reproduces the skills locked in game")
            return 0
        print("MISMATCH: expected %s, got %s" % (sorted(want), sorted(locks)))
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())

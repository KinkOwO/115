#!/usr/bin/env python3
"""Census of the data type 4 container used by contents/2026/apocalypse/etc/*.ctp.

Read-only structural survey. It answers three questions that do not need IDA:
  1. do both .ctp files share one container shape (header + string section)?
  2. how many phase clock groups exist and how are they interleaved with indexes?
  3. which byte offsets carry plausible kinds, i.e. is there one uniform stride?

Usage:
  python next64-ctp-census.py <file.ctp> [<file2.ctp> ...]

Export candidates first with:
  go run ./cmd/pvfinspect -source ../client-build/Script.inner.pvf \
      -files "contents/2026/apocalypse/etc/apocalypse.ctp,contents/2026/apocalypse/etc/dungeonskillinfo.ctp" \
      -output runtime/ctp-census
"""

import collections
import struct
import sys
from pathlib import Path

PHASE_CLOCK = [90.0, 300.0, 300.0, 300.0, 600.0, 600.0]


def header(raw: bytes) -> dict:
    """Read the measured header: version, field count and two declared offsets."""
    return {
        "version": struct.unpack_from("<I", raw, 0)[0],
        "fields": struct.unpack_from("<I", raw, 4)[0],
        "data_end": struct.unpack_from("<I", raw, 0x14)[0],
        "strings_at": struct.unpack_from("<I", raw, 0x1C)[0],
    }


def schema(raw: bytes) -> str:
    """Return the printable ASCII blob that starts the declared string section."""
    start = raw.find(b"[", 0x20)
    end = start
    while end < len(raw) and 0x20 <= raw[end] < 0x7F:
        end += 1
    return raw[start:end].decode("ascii", "replace")


def doubles(raw: bytes) -> list:
    """Every float64 in the file with its offset, deduplicated by offset."""
    out = []
    for off in range(0, len(raw) - 7):
        value = struct.unpack_from("<d", raw, off)[0]
        out.append((off, value))
    return out


def clock_groups(raw: bytes) -> list:
    """Groups matching the phase clock sequence, ignoring interleaved values.

    The source interleaves each clock with its phase index (90, 1, 300, 2, ...),
    so a match advances the expected sequence and anything else is skipped.
    """
    groups = []
    run = []
    hints = []
    for off, value in doubles(raw):
        if value == PHASE_CLOCK[len(run)]:
            if not run:
                hints = []
            run.append(value)
            if len(run) == len(PHASE_CLOCK):
                groups.append({"start": hints[0] if hints else off, "values": list(run)})
                run = []
            continue
        if run:
            hints.append(off)
    return groups


def stride_score(raw: bytes, start: int, step: int, kinds: set) -> tuple:
    total = 0
    known = 0
    for off in range(start, len(raw) - 4, step):
        kind = struct.unpack_from("<I", raw, off)[0]
        total += 1
        if kind in kinds:
            known += 1
    return known, total


def census(path: Path) -> None:
    raw = path.read_bytes()
    head = header(raw)
    blob = schema(raw)
    tags = [t + "]" for t in blob.split("[")[1:]]
    groups = clock_groups(raw)
    print(f"=== {path.name} bytes={len(raw)} version={head['version']} fields={head['fields']} "
          f"data_end={head['data_end']} strings_at={head['strings_at']}")
    print(f"    schema tags={len(tags)} clock groups={len(groups)}")
    for group in groups:
        print(f"    clock group at {group['start']:#06x}: {group['values']}")
    dense = {kind for kind, count in collections.Counter(
        struct.unpack_from("<I", raw, off)[0] for off in range(0, len(raw) - 4)
    ).items() if count >= 2}
    print("    stride scores (kind in the >=2-occurrence set):")
    for step in (8, 12, 16):
        for start in range(0x30, 0x30 + step):
            known, total = stride_score(raw, start, step, dense)
            if total and known / total >= 0.95:
                print(f"      step={step} start={start:#04x} known={known}/{total} "
                      f"= {known / total:.3f}")


def main() -> int:
    if len(sys.argv) < 2:
        print(__doc__)
        return 2
    for arg in sys.argv[1:]:
        census(Path(arg))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

"""Decode a captured monster HP descriptor into plain values.

Every field in these blocks is a guarded pair: the dword at +0 is the stored
form and the dword at +4 is its guard. 145c08420 checks them as

    guard == decoded + 0xc4 + stored

(145c08463..145c0847e and the three copies after it), so a captured pair
yields the decoded value directly, without running the client's decoder:

    decoded = guard - stored - 0xc4   (mod 2^32)

A pair whose guard is zero is unused: the check skips it.

    python decode_monster_stats36.py <run-directory>

Read-only.
"""
import json
import pathlib
import struct
import sys

run = pathlib.Path(sys.argv[1])
rows = []
for line in (run / 'monster-stats.jsonl').read_text(encoding='utf-8').splitlines():
    try:
        r = json.loads(line)
    except Exception:
        continue
    if r.get('kind') == 'monster_hp_descriptor':
        rows.append(r)
print(f'{len(rows)} descriptor snapshots')


def pairs(blob, base):
    out = []
    b = bytes.fromhex(blob)
    for off in range(0, len(b) - 7, 8):
        stored, guard = struct.unpack_from('<II', b, off)
        if guard == 0 and stored == 0:
            out.append((base + off, stored, guard, None))
            continue
        value = (guard - stored - 0xc4) & 0xffffffff
        out.append((base + off, stored, guard, value))
    return out


def show(label, blob, base):
    print(f'  {label}')
    for off, stored, guard, value in pairs(blob, base):
        if value is None:
            print(f'    +0x{off:04x}  unused')
            continue
        f = struct.unpack('<f', struct.pack('<I', value))[0]
        signed = value - 0x100000000 if value >= 0x80000000 else value
        note = ''
        if guard == 0:
            note = '  (guard 0: check skipped)'
        print(f'    +0x{off:04x}  stored={stored:08x} guard={guard:08x} '
              f'-> {value:>10} / {signed:>11} / {f:.7g}{note}')


seen = set()
for r in rows:
    key = (r.get('node_type'), r['raw_hex'], r.get('rate_hex'))
    if key in seen:
        continue
    seen.add(key)
    kind = {1: 'owned/player', 2: 'other actor', 3: 'monster'}.get(r.get('node_type'), 'monster')
    print(f'\nentity {r["entity"]} [{kind}] actor {r["actor"]} team {r["team"]}')
    show('HP descriptor (actor+0x4810)', r['raw_hex'], 0x4810)
    if r.get('rate_hex'):
        show('rate window (actor+0x6800)', r['rate_hex'], 0x6800)

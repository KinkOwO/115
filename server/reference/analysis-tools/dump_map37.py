"""Dump the [monster] section of one map from the runtime catalog, showing each
row's 8 ints and the type-6 options after it, so a parse failure can be read.

    python dump_map37.py 58570

Read-only.
"""
import json
import pathlib
import sys

root = pathlib.Path(__file__).parent.parent / 'dfo-lan'
d = json.loads((root / 'configs/dungeons.next28.json').read_text(encoding='utf-8'))
rec = d['maps'][sys.argv[1]]
cells = rec['cells']

# Print raw cells inside the [monster] section, with types.
active = False
for c in cells:
    if c.get('type') == 3:
        active = c.get('text') == '[monster]'
        if 'monster' in (c.get('text') or ''):
            print(f'=== {c.get("text")}')
        continue
    if not active:
        continue
    print(f'   type={c.get("type")}  {c.get("text", c.get("value"))}')

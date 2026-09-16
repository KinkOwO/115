"""Collect every type-6 token that appears in a [monster] section across the
whole dungeon catalog, so the spawn-option parser can handle every rank and
flag at once instead of failing one room at a time.

The parser (internal/dungeon/session.go) currently handles [fixed], [normal],
[boss], [cinematic], [dummy], [displayhuntdummy]. Anything else -> refusal,
which blocks entering that room.

    python spawn_options37.py

Read-only.
"""
import collections
import json
import pathlib

root = pathlib.Path(__file__).parent.parent / 'dfo-lan'
d = json.loads((root / 'configs/dungeons.generated.json').read_text(encoding='utf-8'))

HANDLED = {'[fixed]', '[normal]', '[boss]', '[cinematic]', '[dummy]', '[displayhuntdummy]'}


def walk(obj):
    """Yield every cell list found under a 'cells' key anywhere in the tree."""
    if isinstance(obj, dict):
        if 'cells' in obj and isinstance(obj['cells'], list):
            yield obj['cells']
        for v in obj.values():
            yield from walk(v)
    elif isinstance(obj, list):
        for v in obj:
            yield from walk(v)


opts = collections.Counter()
for cells in walk(d):
    active = False
    for c in cells:
        if not isinstance(c, dict):
            continue
        if c.get('type') == 3:
            active = c.get('text') == '[monster]'
            continue
        if active and c.get('type') == 6:
            opts[c.get('text')] += 1

print('type-6 tokens inside [monster] sections:')
for t, n in opts.most_common():
    mark = 'ok' if t in HANDLED else '<< UNHANDLED'
    print(f'  {n:>6}  {t!r:<28} {mark}')

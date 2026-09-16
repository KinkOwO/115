"""Every distinct bracketed type-6 token in the whole dungeon catalog, with a
count, so no monster-row spawn option is missed by the parser.

    python all_tokens37.py

Read-only.
"""
import collections
import json
import pathlib

root = pathlib.Path(__file__).parent.parent / 'dfo-lan'
raw = (root / 'configs/dungeons.generated.json').read_text(encoding='utf-8')
d = json.loads(raw)
opts = collections.Counter()


def walk(o):
    if isinstance(o, dict):
        if o.get('type') == 6 and isinstance(o.get('text'), str) and o['text'].startswith('['):
            opts[o['text']] += 1
        for v in o.values():
            walk(v)
    elif isinstance(o, list):
        for v in o:
            walk(v)


walk(d)
for t, n in opts.most_common():
    print(f'  {n:>6}  {t}')

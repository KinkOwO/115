"""Which dungeon catalog does the runtime use, and every monster spawn option
in it. The runtime visited maps (58562+, 76161+) and dungeons (6,7,9) that are
NOT in dungeons.generated.json, so it must load dungeons.next28.json. Scan both
for every type-6 token inside a [monster] section so the parser handles all of
them at once.

    python spawn_scan_next28.py

Read-only.
"""
import collections
import json
import pathlib

root = pathlib.Path(__file__).parent.parent / 'dfo-lan'
HANDLED = {'[fixed]', '[normal]', '[champion]', '[boss]', '[cinematic]', '[dummy]', '[displayhuntdummy]'}

for name in ('dungeons.generated.json', 'dungeons.next28.json', 'tutorial-dungeons.current36.json'):
    d = json.loads((root / 'configs' / name).read_text(encoding='utf-8'))
    maps = d.get('maps', {})
    dungeons = sorted(int(k) for k in d.get('dungeons', {}))
    opts = collections.Counter()
    for rec in maps.values():
        cells = rec.get('cells', [])
        active = False
        for c in cells:
            if c.get('type') == 3:
                active = c.get('text') == '[monster]'
                continue
            if active and c.get('type') == 6:
                opts[c.get('text')] += 1
    print(f'\n== {name}: {len(maps)} maps, dungeons {dungeons[:12]}{"..." if len(dungeons)>12 else ""}')
    for t, n in opts.most_common():
        mark = '' if t in HANDLED else '   << UNHANDLED'
        print(f'   {n:>6}  {t}{mark}')

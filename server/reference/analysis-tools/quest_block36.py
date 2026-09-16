"""Why do quests fail to settle? Audit the two refusals seen live.

  quest 2109  -> "unsupported quest difficulty"   (progression/quest.go)
  quest 21650 -> "equipment absent from source"   (inventory/equipment.go)

Both are source-coverage questions, so count them over the whole catalog
instead of generalising from one sample.

    python quest_block36.py [quest-id ...]

Read-only.
"""
import collections
import json
import pathlib
import sys

root = pathlib.Path(__file__).parent.parent / 'dfo-lan'
quests = json.loads((root / 'configs/quests.generated.json').read_text(encoding='utf-8'))['quests']
equip = json.loads((root / 'configs/equipment.current35.json').read_text(encoding='utf-8'))
items = equip.get('items', equip.get('rows', equip))
known = ({int(k) for k in items} if isinstance(items, dict)
         else {r.get('ID', r.get('id')) for r in items})
print(f'{len(quests)} quests, {len(known)} equipment templates in the catalog')


def section(cells, name):
    """Exactly progression/experience.go:section - every non-marker cell that
    follows the named marker, until another marker changes the state."""
    out = []
    on = False
    for c in cells:
        if c.get('type') == 3:
            on = c.get('text') == name
            continue
        if on:
            out.append(c)
    return out


shapes = collections.Counter()
values = collections.Counter()
missing_equipment = collections.Counter()
examples = {}
for qid, q in quests.items():
    cells = q.get('script', {}).get('cells', [])
    d = section(cells, '[difficulty]')
    if len(d) == 1 and d[0].get('type') == 6:
        shapes['ok: one text cell'] += 1
        values[d[0].get('text')] += 1
    elif not d:
        shapes['absent'] += 1
        examples.setdefault('absent', qid)
    else:
        key = f'{len(d)} cells, types ' + ','.join(str(c.get("type")) for c in d[:4])
        shapes[key] += 1
        examples.setdefault(key, qid)

    rtype = section(cells, '[reward type]')
    if any(c.get('text') == '[item]' for c in rtype):
        for c in section(cells, '[reward int data]'):
            v = c.get('value')
            if c.get('type') == 0 and v and v not in known and v > 1000:
                missing_equipment[v] += 1

print('\n[difficulty] shapes:')
for k, n in shapes.most_common():
    ex = f'   e.g. quest {examples[k]}' if k in examples else ''
    print(f'  {n:>5}  {k}{ex}')
print('\n[difficulty] values in use:')
for k, n in values.most_common():
    print(f'  {n:>5}  {k}')
print(f'\nreward ids not in the equipment catalog: {len(missing_equipment)} distinct, '
      f'{sum(missing_equipment.values())} references')
for v, n in missing_equipment.most_common(15):
    print(f'  {v:>12} x{n}')

for arg in sys.argv[1:]:
    q = quests.get(arg)
    if not q:
        print(f'\nquest {arg}: absent')
        continue
    cells = q['script']['cells']
    print(f'\nquest {arg}  {q["script"]["path"]}')
    for tag in ('[difficulty]', '[reward type]', '[reward int data]', '[type]', '[level]'):
        s = section(cells, tag)
        print(f'  {tag:<20} ' + ', '.join(
            str(c.get('text', c.get('reference', c.get('value')))) for c in s))

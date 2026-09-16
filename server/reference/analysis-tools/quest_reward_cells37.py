"""Print the raw reward cells of specific quests, and the equip requirements
of specific equipment templates. Answers two live questions:

  * do these quests carry a gold reward we are dropping?
  * why does the client refuse this gear (job / level / grow)?

    python quest_reward_cells37.py --quests 3147 3148 4873 --equip 404030090 100261068 27603

Read-only.
"""
import json
import pathlib
import sys

root = pathlib.Path(__file__).parent.parent / 'dfo-lan'
quests = json.loads((root / 'configs/quests.generated.json').read_text(encoding='utf-8'))['quests']
equip = json.loads((root / 'configs/equipment.current37.json').read_text(encoding='utf-8'))['rows']
byid = {r['ID']: r for r in equip}

mode = None
qs, es = [], []
for a in sys.argv[1:]:
    if a == '--quests':
        mode = qs
    elif a == '--equip':
        mode = es
    elif mode is not None:
        mode.append(int(a))


def sec(cells, name):
    out, on = [], False
    for c in cells:
        if c.get('type') == 3:
            on = c.get('text') == name
            continue
        if on:
            out.append(c)
    return out


for qid in qs:
    q = quests.get(str(qid))
    if not q:
        print(f'quest {qid}: absent'); continue
    cells = q['script']['cells']
    print(f'\nquest {qid}')
    for tag in ('[reward type]', '[reward int data]', '[reward selection int data]',
                '[reward gold]', '[gold]', '[reward exp]'):
        s = sec(cells, tag)
        if s:
            print(f'  {tag}: ' + ', '.join(str(c.get('text', c.get('value'))) for c in s))

for eid in es:
    r = byid.get(eid)
    print(f'\nequip {eid}')
    if not r:
        print('  absent'); continue
    for tag in ('[equipment type]', '[usable job]', '[usable grow type]',
                '[minimum level]', '[attach type]', '[rarity]'):
        v = r['Fields'].get(tag, [])
        print(f'  {tag}: ' + ', '.join(str(c.get('text', c.get('value'))) for c in v))

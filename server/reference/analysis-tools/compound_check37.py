"""For every compound-[type] quest, show its first token, last token and the
int-data shape, so a "use the first token as the kind" change can be checked:
the int-data must fit the FIRST objective's decoder.

    python compound_check37.py

Read-only.
"""
import collections
import json
import pathlib

root = pathlib.Path(__file__).parent.parent / 'dfo-lan'
quests = json.loads((root / 'configs/quests.generated.json').read_text(encoding='utf-8'))['quests']


def sec(cells, name):
    out, on = [], False
    for c in cells:
        if c.get('type') == 3:
            on = c.get('text') == name
            continue
        if on:
            out.append(c)
    return out


groups = collections.defaultdict(list)
for qid, q in quests.items():
    cells = q['script']['cells']
    types = [c.get('text') for c in sec(cells, '[type]') if c.get('type') == 6]
    if len(types) <= 1:
        continue
    intdata = sec(cells, '[int data]')
    shape = (types[0], types[-1], len(intdata))
    groups[shape].append((qid, [c.get('value') for c in intdata]))

for (first, last, n), rows in sorted(groups.items(), key=lambda kv: -len(kv[1])):
    print(f'\nfirst={first!r} last={last!r}  int-data cells={n}  ({len(rows)} quests)')
    for qid, vals in rows[:3]:
        print(f'   {qid}: {vals}')

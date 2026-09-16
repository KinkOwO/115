"""Histogram of quest objective kinds, split by whether this build settles them.

Implemented kinds (internal/quest/progress.go): [meet npc], [clear map],
[reach the range], [seek n meet npc]. Everything else is "objective type not
implemented" and cannot be offered. This shows which unimplemented kind, if
built next, unblocks the most quests - and how many are low level.

    python quest_kinds37.py

Read-only.
"""
import collections
import json
import pathlib

root = pathlib.Path(__file__).parent.parent / 'dfo-lan'
quests = json.loads((root / 'configs/quests.generated.json').read_text(encoding='utf-8'))['quests']

IMPLEMENTED = {'[meet npc]', '[clear map]', '[reach the range]', '[seek n meet npc]'}


def sec(cells, name):
    out, on = [], False
    for c in cells:
        if c.get('type') == 3:
            on = c.get('text') == name
            continue
        if on:
            out.append(c)
    return out


total = collections.Counter()
low = collections.Counter()      # min level <= 60
struct_shape = collections.defaultdict(collections.Counter)
for q in quests.values():
    cells = q['script']['cells']
    kinds = [c.get('text') for c in sec(cells, '[type]') if c.get('type') == 6]
    kind = kinds[0] if kinds else '(no type)'
    total[kind] += 1
    lvl = sec(cells, '[level]')
    minlv = lvl[0]['value'] if len(lvl) == 2 and lvl[0].get('type') == 0 else 9999
    if minlv <= 60:
        low[kind] += 1
    intdata = sec(cells, '[int data]')
    struct_shape[kind][len(intdata)] += 1

print(f'{sum(total.values())} quests total\n')
print(f'{"kind":<28} {"total":>6} {"lo(<=60)":>8}  impl?  int-data cell counts')
for kind, n in total.most_common():
    mark = 'YES' if kind in IMPLEMENTED else ' - '
    shapes = ', '.join(f'{k}:{v}' for k, v in sorted(struct_shape[kind].items()))
    print(f'{kind:<28} {n:>6} {low[kind]:>8}   {mark}   {shapes[:60]}')

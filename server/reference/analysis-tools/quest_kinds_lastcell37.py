"""Histogram of quest kinds using the LAST [type] token - the semantics the Go
catalog parser actually uses (internal/catalog/quests.go sets d.Kind to the
last type-6 cell). The first-cell histogram in quest_kinds37.py mis-attributed
compound-type quests like 21029 ([look cinematic] + arrive in town).

    python quest_kinds_lastcell37.py

Read-only.
"""
import collections
import json
import pathlib

root = pathlib.Path(__file__).parent.parent / 'dfo-lan'
quests = json.loads((root / 'configs/quests.generated.json').read_text(encoding='utf-8'))['quests']
IMPL = {'[meet npc]', '[clear map]', '[reach the range]', '[seek n meet npc]', '[look cinematic]'}


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
low = collections.Counter()
compound = collections.Counter()
for q in quests.values():
    cells = q['script']['cells']
    types = [c.get('text') for c in sec(cells, '[type]') if c.get('type') == 6]
    kind = types[-1] if types else '(none)'
    total[kind] += 1
    if len(types) > 1:
        compound[' + '.join(types)] += 1
    lvl = sec(cells, '[level]')
    minlv = lvl[0]['value'] if len(lvl) == 2 and lvl[0].get('type') == 0 else 9999
    if minlv <= 60:
        low[kind] += 1

print(f'{sum(total.values())} quests, by LAST [type] token\n')
print(f'{"kind":<28} {"total":>6} {"lo<=60":>7}  impl?')
for kind, n in total.most_common():
    print(f'{kind:<28} {n:>6} {low[kind]:>7}   {"YES" if kind in IMPL else " - "}')

print(f'\n{sum(compound.values())} quests have a compound [type]; top shapes:')
for shape, n in compound.most_common(15):
    print(f'  {n:>4}  {shape}')

"""Find low-level, freely-equippable archer gear - one per body slot - so a
known-good set can be granted to test whether equipping works at all.

A candidate must be [free] attach type, usable by [all] or [archer], rarity
<= 1, minimum level <= 9, have a real durability or be an accessory, and carry
no [impossible contents] equip restriction.

    python basic_archer_gear37.py

Read-only.
"""
import json
import pathlib

root = pathlib.Path(__file__).parent.parent / 'dfo-lan'
equip = json.loads((root / 'configs/equipment.current37.json').read_text(encoding='utf-8'))['rows']


def field(r, tag):
    return r['Fields'].get(tag, [])


def text1(r, tag):
    v = field(r, tag)
    return v[0].get('text') if v else None


best = {}
for r in equip:
    et = field(r, '[equipment type]')
    if not et:
        continue
    slot = et[0].get('text')
    jobs = [c.get('text') for c in field(r, '[usable job]')]
    if not any(j in ('[all]', '[archer]') for j in jobs):
        continue
    if text1(r, '[attach type]') != '[free]':
        continue
    rr = field(r, '[rarity]')
    if not rr or rr[0].get('value', 9) > 1:
        continue
    ml = field(r, '[minimum level]')
    minlv = ml[0].get('value') if ml else 0
    if minlv > 9:
        continue
    cur = best.get(slot)
    if cur is None or minlv > cur[1]:
        best[slot] = (r['ID'], minlv)

print('one freely-equippable [all]/[archer] item per slot, level <= 9:')
for slot, (rid, lv) in sorted(best.items()):
    print(f'  {slot:<14} {rid}  (min level {lv})')
print('\ngrant string:',
      ','.join(f'{rid}' for rid, _ in best.values()))

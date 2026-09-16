"""Find which map(s) in the runtime dungeon catalog carry a [champion] spawn
option, and print the token type + the monster-row context, so the parser fix
matches the real data exactly.

    python find_champion_map37.py

Read-only.
"""
import json
import pathlib

root = pathlib.Path(__file__).parent.parent / 'dfo-lan'
d = json.loads((root / 'configs/dungeons.generated.json').read_text(encoding='utf-8'))
maps = d.get('maps', {})
print('total maps in catalog:', len(maps))

for mid, rec in maps.items():
    cells = rec.get('cells', [])
    for i, c in enumerate(cells):
        if c.get('text') == '[champion]':
            print(f'\nmap {mid}: [champion] at cell {i}, type={c.get("type")}')
            lo = max(0, i - 10)
            for j in range(lo, min(len(cells), i + 4)):
                cc = cells[j]
                mark = '  <<' if j == i else ''
                print(f'   [{j}] type={cc.get("type")} '
                      f'{cc.get("text", cc.get("value"))}{mark}')

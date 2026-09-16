import json
import pathlib

root = pathlib.Path(__file__).parent.parent / 'dfo-lan'
d = json.loads((root / 'configs/progression.next25.json').read_text(encoding='utf-8'))
cells = d['scripts']['n_quest/questparameter.etc']['cells']
on = None
out = {}
for c in cells:
    if c.get('type') == 3:
        on = c.get('text')
        out.setdefault(on, [])
        continue
    if on is not None:
        out[on].append(c.get('value'))
for tag in ('[exp reward table]', '[gold reward table]'):
    v = out.get(tag, [])
    print(tag, 'len', len(v))
    print('  first 30:', v[:30])
    print('  last 6:', v[-6:])

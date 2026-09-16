"""Print the quest parameter table sections this build actually ships."""
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
        out[on].append(c.get('text', c.get('value')))
print('sections:', list(out))
for tag, v in out.items():
    if 'difficulty' in tag or 'penalty' in tag:
        print(f'\n{tag} ({len(v)} cells)')
        print('  ', v[:120])

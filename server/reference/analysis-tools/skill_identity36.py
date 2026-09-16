"""Print what a skill id actually is in this build's source catalog.

    python skill_identity36.py 179 511 3 7 ...

Read-only lookup in configs/skills.next27.json.
"""
import json
import pathlib
import sys

root = pathlib.Path(__file__).parent.parent / 'dfo-lan'
rows = json.loads((root / 'configs/skills.next27.json').read_text(encoding='utf-8'))['rows']
by_id = {}
for r in rows:
    by_id.setdefault(r['ID'], []).append(r)

for arg in sys.argv[1:]:
    sid = int(arg)
    for r in by_id.get(sid, []):
        f = r.get('Fields', {})

        def one(tag):
            v = f.get(tag, [])
            return ' '.join(str(x.get('text', x.get('reference', x.get('value')))) for x in v)

        print(f'id {sid:<5} job {r.get("Job")}  name={r.get("Name", one("[name]"))!r}')
        print(f'      type={one("[type]")}  grow={one("[skill fitness growtype]")}  '
              f'cap={one("[growtype maximum level]")}  need={one("[need level]")}')
        print(f'      path={r.get("Path", "")}')
    if sid not in by_id:
        print(f'id {sid:<5} absent from the catalog')

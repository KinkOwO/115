"""Does "growtype list contains 0" really mean "learnable before advancing"?

Swordman is untouched source: its unadvanced page carries 27 cells. If the
rule is right, the skills it places there are exactly the Job 0 skills whose
[skill fitness growtype] contains 0. If the rule is wrong, the two sets will
not line up, and the Archer conclusion drawn from it has to be thrown out.

    python validate_grow0_rule37.py

Read-only.
"""
import json
import pathlib

root = pathlib.Path(__file__).parent.parent / 'dfo-lan'
defs = json.loads((root / 'configs/skills.next27.json').read_text(encoding='utf-8'))['rows']


def eligible(job):
    out = set()
    for d in defs:
        if d['Job'] != job:
            continue
        f = d['Fields']
        types = f.get('[type]', [])
        if len(types) != 1 or types[0].get('text') not in ('[active]', '[passive]'):
            continue
        grow = [v['value'] for v in f.get('[skill fitness growtype]', []) if v['type'] == 0]
        cap = [v['value'] for v in f.get('[growtype maximum level]', []) if v['type'] == 0]
        if cap and cap[0] <= 0:
            continue
        if (grow and 0 in grow) or (not grow and cap and cap[0] > 0):
            out.add(d['ID'])
    return out


def base_page(dump):
    cells = json.loads((root / 'runtime/skill-layout35' / dump).read_text())
    out, current, row, ids = [], None, None, set()
    for i, c in enumerate(cells):
        t = c.get('text')
        if t == '[character job]':
            current = cells[i + 2]['text']
        elif t == '[skill info]':
            row = {}
        elif t == '[index]' and row is not None:
            row['id'] = cells[i + 1]['value']
        elif t == '[cell pos]' and row is not None:
            row['xy'] = True
        elif t == '[/skill info]' and row is not None:
            if current == 'none' and row.get('xy'):
                ids.add(row['id'])
            row = None
    return ids


for job, dump, name in ((0, '01-swordman_sp.co.tokens.json', 'swordman'),
                        (16, '00-archer_sp.co.tokens.json', 'archer')):
    el = eligible(job)
    page = base_page(dump)
    print(f'== {name} (job {job})')
    print(f'   unadvanced page cells : {len(page)}')
    print(f'   growtype-0 skills     : {len(el)}')
    print(f'   on the page but NOT growtype-0 : {sorted(page - el)}')
    print(f'   growtype-0 but NOT on the page : {len(el - page)} -> {sorted(el - page)}')
    print()

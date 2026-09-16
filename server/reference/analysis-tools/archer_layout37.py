"""Rebuild the Archer base-job layout patch from the source's own evidence.

35 added three cells to the unadvanced page - 3, 7 and 511 - by copying
coordinates out of the "muse" advancement section. Comparing against an
untouched job shows two of those were wrong:

  * Swordman's unadvanced page carries 27 cells over rows y=0..6; Archer's
    carries 7, all on y=0. The seven are identical in both files, same ids and
    same x positions: that row is the shared common row.
  * 511 (armormasteryetc) sits at x=0,y=1 in EVERY section of both files,
    including Swordman's unadvanced page. Archer's unadvanced page is the only
    place it is missing, and 35 happened to place it at exactly 0,1.
  * 3 (CrackArrow) and 7 (RisingMoon) sit at different coordinates in every
    advancement section (muse 4,1 / traveler 5,1 / hunter 4,1 / vigilante 3,1
    / chimera 6,1), and Swordman declares its own 3 and 7 as unplaced. They do
    not belong on an unadvanced page, and the coordinates 35 gave them were
    borrowed from one advancement section out of five.

So the rule here is what the source can actually support: a skill is added
only when every section that places it agrees on one coordinate. That admits
511 and rejects 3 and 7, with no judgement call left over.

UI only. No granted skills, stats, costs or advancement changes. Writes a new
directory; never touches the client or the original PVF.
"""
from pathlib import Path
import json
import struct
import hashlib

root = Path(__file__).parent.parent / 'dfo-lan'
cells = json.loads((root / 'runtime/skill-layout35/00-archer_sp.co.tokens.json').read_text())


sections = []
current = None
row = None
for i, c in enumerate(cells):
    t = c.get('text')
    if t == '[character job]':
        current = {'grow': cells[i + 2]['text'], 'rows': [], 'start': i}
    if t == '[skill info]':
        row = {'start': i}
    if t == '[index]' and row is not None:
        row['id'] = cells[i + 1]['value']
    if t == '[cell pos]' and row is not None:
        row['xy'] = [cells[i + 1]['value'], cells[i + 2]['value']]
    if t == '[/skill info]' and row is not None:
        row['cells'] = cells[row['start']:i + 1]
        row.setdefault('xy', None)
        current['rows'].append(row)
        row = None
    if t == '[/character job]' and current is not None:
        current['end'] = i
        sections.append(current)
        current = None

base = next(s for s in sections if s['grow'] == 'none')
existing = {r['id'] for r in base['rows']}
occupied = {tuple(r['xy']) for r in base['rows'] if r['xy']}

# Where does each skill sit, across every advancement section?
placements = {}
for s in sections:
    if s['grow'] == 'none':
        continue
    for r in s['rows']:
        if r['id'] in existing or not r['xy']:
            continue
        placements.setdefault(r['id'], []).append((s['grow'], r))

# What may appear on the unadvanced page is a question about the skill, not
# about the layout: [skill fitness growtype] lists six entries for this job,
# one unadvanced plus five advancements, and 0 is the unadvanced one. A skill
# whose list contains 0 is learnable before advancing, so it belongs on this
# page. 3 (CrackArrow) and 7 (RisingMoon) both read "0 1 2 3 4 5".
defs = json.loads((root / 'configs/skills.next27.json').read_text())['rows']
eligible = set()
for d in defs:
    if d['Job'] != 16:
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
        eligible.add(d['ID'])

added = []
rejected = {}
for idx, found in sorted(placements.items()):
    if idx not in eligible:
        rejected[idx] = ['not learnable before advancing']
        continue
    # The cell has to come from the source. Take the first section that places
    # it somewhere the unadvanced page has not already used.
    free = [(g, r) for g, r in found if tuple(r['xy']) not in occupied]
    if not free:
        rejected[idx] = ['every source cell collides with an existing one']
        continue
    found = free
    xy = found[0][1]['xy']
    tags = {c.get('text'): c for c in found[0][1]['cells'] if c['type'] == 3}
    v = ([tags['[skill info]'], tags['[index]'], {'type': 0, 'value': idx},
          tags['[cell pos]']]
         + [{'type': 0, 'value': x} for x in xy]
         + [tags['[/skill info]']])
    occupied.add(tuple(xy))
    added.append({'id': idx, 'xy': xy, 'sections': [g for g, _ in found], 'cells': v})


patched = cells[:base['end']] + [c for r in added for c in r['cells']] + cells[base['end']:]


def encode(cs):
    return b''.join(struct.pack('<Bi', c['type'], c['value']) for c in cs)


raw = encode(cells)
expected = (root / 'runtime/skill-layout35/00-archer_sp.co.bin').read_bytes()
if raw != expected:
    raise ValueError('token round-trip no longer reproduces the source entry')
new = encode(patched)

dest = root / 'runtime/archer-layout-patch37'
dest.mkdir(exist_ok=True)
(dest / 'archer_sp.co.bin').write_bytes(new)
manifest = {
    'source_entry': 'clientonly/skilltree/archer_sp.co',
    'source_sha256': hashlib.sha256(raw).hexdigest(),
    'patched_sha256': hashlib.sha256(new).hexdigest(),
    'original_common_ids': sorted(existing),
    'added': [{k: v for k, v in r.items() if k != 'cells'} for r in added],
    'rejected': {k: v for k, v in rejected.items()
                 if v[0] != 'not learnable before advancing'},
    'rejected_not_unadvanced': sum(1 for v in rejected.values()
                                   if v[0] == 'not learnable before advancing'),
    'eligible_not_in_any_layout': sorted(eligible - existing - set(placements)),
    'rule': 'add a skill when its [skill fitness growtype] contains 0, the '
            'unadvanced grow type; take its cell from the first source '
            'section that places it somewhere still free',
    'scope': 'UI-only base-job layout; no granted skills, stats, costs or '
             'advancement changes',
}
(dest / 'manifest.json').write_text(json.dumps(manifest, indent=2))
print(json.dumps(manifest, indent=2))

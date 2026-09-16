"""Compare base-job skill-tree pages across jobs.

The archer page was reported as wrong. 35 added three cells to it, and the
question left open was whether the page is missing cells, has them in the
wrong place, or has cells it should not. An untouched job's own layout
answers that: if every job's unadvanced page carries a comparable number of
cells spread over the level rows, a page with seven cells crowded into one
row is under-populated, not misplaced.

Read-only. Parses the token dumps already exported under
runtime/skill-layout35/.

    python compare_skill_layout36.py
"""
import json
import pathlib

root = pathlib.Path(__file__).parent.parent / 'dfo-lan'
dumps = sorted((root / 'runtime/skill-layout35').glob('*.tokens.json'))


def sections(cells):
    """[character job] sections, each with its grow type and cell rows."""
    out, current, row = [], None, None
    for i, c in enumerate(cells):
        text = c.get('text')
        if text == '[character job]':
            current = {'grow': cells[i + 2]['text'], 'rows': []}
        elif text == '[skill info]':
            row = {}
        elif text == '[index]' and row is not None:
            row['id'] = cells[i + 1]['value']
        elif text == '[cell pos]' and row is not None:
            row['xy'] = (cells[i + 1]['value'], cells[i + 2]['value'])
        elif text == '[/skill info]' and row is not None:
            # A few entries carry an index with no [cell pos]: they are
            # declared but not placed on the page.
            row.setdefault('xy', None)
            current['rows'].append(row)
            row = None
        elif text == '[/character job]' and current is not None:
            out.append(current)
            current = None
    return out


for dump in dumps:
    cells = json.loads(dump.read_text(encoding='utf-8'))
    print(f'===== {dump.name}')
    for s in sections(cells):
        rows = s['rows']
        if not rows:
            print(f'  grow {s["grow"]:<12} 0 cells')
            continue
        placed = [r for r in rows if r['xy']]
        ys = sorted({r['xy'][1] for r in placed})
        marker = '   <-- base job page' if s['grow'] == 'none' else ''
        print(f'  grow {s["grow"]:<12} {len(rows):>3} cells   '
              f'rows y={ys}{marker}')
        if s['grow'] == 'none':
            for r in sorted(placed, key=lambda r: (r['xy'][1], r['xy'][0])):
                print(f'        skill {r["id"]:<6} at x={r["xy"][0]} y={r["xy"][1]}')
    # Where do the skills that have no base-page cell actually live?
    wanted = [172, 179, 181, 182, 184, 185, 191, 192, 193, 194,
              200, 201, 481, 504, 505, 506, 3, 7, 511]
    where = {}
    for s in sections(cells):
        for r in s['rows']:
            if r['id'] in wanted:
                where.setdefault(r['id'], []).append(
                    f'{s["grow"]}{"" if r["xy"] else "(unplaced)"}'
                    + (f'@{r["xy"][0]},{r["xy"][1]}' if r['xy'] else ''))
    print('  placement of the skills in question:')
    for sid in wanted:
        print(f'    {sid:<5} {", ".join(where.get(sid, ["absent from this file"]))}')
    print()

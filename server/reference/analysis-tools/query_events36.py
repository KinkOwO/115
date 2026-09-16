"""Small read-only queries over one run's events.jsonl.

    python query_events36.py <run-dir> kinds
    python query_events36.py <run-dir> tail [n]
    python query_events36.py <run-dir> id <command> [n]
    python query_events36.py <run-dir> grep <substring> [n]
"""
import json
import pathlib
import sys
from collections import Counter

run = pathlib.Path(sys.argv[1])
mode = sys.argv[2] if len(sys.argv) > 2 else 'kinds'
rows = []
for line in (run / 'events.jsonl').read_text(encoding='utf-8', errors='replace').splitlines():
    try:
        rows.append(json.loads(line))
    except Exception:
        pass
print(f'{len(rows)} events')


def show(r):
    t = r.get('time', '')[11:23]
    body = {k: v for k, v in r.items() if k not in ('time', 'hex', 'plain_hex')}
    extra = ''
    if r.get('plain_hex'):
        extra = ' plain=' + r['plain_hex'][:160]
    print(f'{t:>13} {json.dumps(body, ensure_ascii=False)[:240]}{extra}')


if mode == 'kinds':
    for kind, n in Counter(r.get('kind') for r in rows).most_common():
        print(f'  {n:>6}  {kind}')
    print('\nclient_frame command ids:')
    ids = Counter(r.get('id') for r in rows if r.get('kind') == 'client_frame')
    print('  ' + ', '.join(f'{k}x{v}' for k, v in sorted(ids.items(), key=lambda x: -x[1])))
elif mode == 'tail':
    n = int(sys.argv[3]) if len(sys.argv) > 3 else 40
    for r in rows[-n:]:
        show(r)
elif mode == 'id':
    want = int(sys.argv[3])
    n = int(sys.argv[4]) if len(sys.argv) > 4 else 20
    hit = [r for r in rows if r.get('id') == want]
    print(f'{len(hit)} with id={want}')
    for r in hit[-n:]:
        show(r)
elif mode == 'grep':
    needle = sys.argv[3]
    n = int(sys.argv[4]) if len(sys.argv) > 4 else 30
    hit = [r for r in rows if needle in json.dumps(r, ensure_ascii=False)]
    print(f'{len(hit)} matching {needle!r}')
    for r in hit[-n:]:
        show(r)

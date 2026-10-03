import json, sys
path, want = sys.argv[1], sys.argv[2]
for ln in open(path, encoding='utf-8'):
    try:
        e = json.loads(ln)
    except Exception:
        continue
    if e.get('kind') != 'client_frame' or e.get('peer') != want:
        continue
    print(e['time'][11:19], 'id=%-5d' % e['id'], 'op=%-5d' % e['type'], e.get('plain_hex', '')[:40])

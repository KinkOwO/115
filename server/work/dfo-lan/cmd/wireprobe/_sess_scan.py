import json, sys, collections

path = sys.argv[1]
peers = {}   # peer -> list of events
accepts = []
for ln in open(path, encoding='utf-8'):
    try:
        e = json.loads(ln)
    except Exception:
        continue
    k = e.get('kind')
    if k == 'accept':
        accepts.append(e)
    p = e.get('peer')
    if p:
        peers.setdefault(p, []).append(e)

print(f"accepts={len(accepts)} peers={len(peers)}")
for ts in accepts:
    print("ACCEPT", ts['time'], ts['peer'])

for p, evs in peers.items():
    frames = [e for e in evs if e.get('kind') == 'client_frame']
    if not frames:
        continue
    t0, t1 = frames[0]['time'][11:19], frames[-1]['time'][11:19]
    ids = [e['id'] for e in frames]
    ops = collections.Counter(e['type'] for e in frames)
    print(f"\nPEER {p} frames={len(frames)} first={t0} last={t1} span={ids[-1]-ids[0]}")
    print("  opcounts:", dict(sorted(ops.items(), key=lambda kv: -kv[1])[:12]))
    # tail: last 12 client frames with time gaps
    print("  tail:")
    prev = None
    for e in frames[-14:]:
        gap = ''
        if prev:
            import datetime as dt
            a = dt.datetime.fromisoformat(prev['time'][:26].rstrip('Z') if prev['time'].endswith('Z') else prev['time'][:26]); b = dt.datetime.fromisoformat(e['time'][:26].rstrip('Z') if e['time'].endswith('Z') else e['time'][:26])
            g = (b-a).total_seconds()
            if g > 3: gap = f' (+{g:.0f}s)'
        print(f"    {e['time'][11:19]} id={e['id']} op={e['type']} hex={e.get('plain_hex','')[:28]}{gap}")
        prev = e

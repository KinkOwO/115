import json, collections

frames = json.load(open(r'D:\115us\analysis-tools\output\ispins_switch_frames.json', encoding='utf-8'))
print("total", len(frames), "span_sec", (frames[-1]['ts']-frames[0]['ts'])/1000.0)

s2c = collections.Counter(f['op'] for f in frames if f['dir']=='s2c')
c2s = collections.Counter(f['op'] for f in frames if f['dir']=='c2s')
print("\nS2C ops:", sorted(s2c.items(), key=lambda kv:-kv[1])[:20])
print("\nC2S ops:", sorted(c2s.items(), key=lambda kv:-kv[1])[:20])

# heartbeat 2127 pattern
hb = [f for f in frames if f['op']==2127]
print("\nop2127 frames:", len(hb), "dirs:", collections.Counter(f['dir'] for f in hb))
if hb:
    t0=hb[0]['ts']
    print("first 2127 at t+%.1fs rel start" % ((t0-frames[0]['ts'])/1000))
    print("last  2127 at t+%.1fs" % ((hb[-1]['ts']-frames[0]['ts'])/1000))
    gaps=[(hb[i+1]['ts']-hb[i]['ts'])/1000 for i in range(len(hb)-1)]
    if gaps: print("2127 gap min/med/max: %.1f/%.1f/%.1f s" % (min(gaps), sorted(gaps)[len(gaps)//2], max(gaps)))

# idle segment: find largest gaps between consecutive frames
gaps = sorted((((frames[i+1]['ts']-frames[i]['ts'])/1000, i) for i in range(len(frames)-1)), reverse=True)[:6]
for g, i in gaps:
    a, b = frames[i], frames[i+1]
    print(f"\ngap {g:.0f}s after frame {a['id']} op={a['op']}({a.get('op_name','')}) dir={a['dir']} at t+{(a['ts']-frames[0]['ts'])/1000:.0f}s")
    print(f"  next frame {b['id']} op={b['op']}({b.get('op_name','')}) dir={b['dir']} len={b['len']}")
    # what did s2c send periodically around there: dump frames in [a.id+1, a.id+12]
    for f in frames[i+1:i+13]:
        print(f"    f{f['id']} {f['dir']} op={f['op']}({f.get('op_name','')}) len={f['len']} t+{(f['ts']-frames[0]['ts'])/1000:.0f}s hex={f.get('plain_hex','')[:24]}")

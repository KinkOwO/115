import json, collections

frames = json.load(open(r'D:\115us\analysis-tools\output\ispins_switch_frames.json', encoding='utf-8'))

# all 1719/1706 frames with timestamps
print("=== SEC_PING_CHECK pattern ===")
pings = [(f['id'], (f['ts']-frames[0]['ts'])/1000, f['dir'], f['op'], f.get('plain_hex','')[:28]) for f in frames if f['op'] in (1719, 1706)]
for p in pings: print(f"  f{p[0]} t+{p[1]:.0f}s {p[2]} op={p[3]} {p[4]}")
if len(pings) > 2:
    t = [p[1] for p in pings if p[3]==1719]
    print("  1719 intervals:", [round(t[i+1]-t[i]) for i in range(len(t)-1)])

# frames up to and around the second c2s op=4 (character select)
sel = [f for f in frames if f['dir']=='c2s' and f['op']==4]
print("\n=== c2s op4 (select) frames at:", [(f['id'], round((f['ts']-frames[0]['ts'])/1000)) for f in sel])

# dump first 75 frames
print("\n=== first 75 frames ===")
for f in frames[:75]:
    print(f"f{f['id']:4d} t+{(f['ts']-frames[0]['ts'])/1000:6.1f}s {f['dir']} op={f['op']:5d} {f.get('op_name','')[:42]:42s} len={f['len']:5d} {f.get('plain_hex','')[:20]}")

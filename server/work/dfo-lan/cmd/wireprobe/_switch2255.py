import json

frames = json.load(open(r'D:\115us\analysis-tools\output\ispins_switch_frames.json', encoding='utf-8'))
print('total', len(frames))
for i, f in enumerate(frames):
    if f.get('op') in (2254, 2255, 2256, 1336, 637, 706, 781, 782, 537) and f.get('dir') == 's2c':
        ph = f.get('plain_hex') or ''
        conn = f.get('conn')
        print(f"f{i} conn={conn} op={f['op']} {f.get('op_name','')[:40]} len={f.get('len')} {ph[:80]}")

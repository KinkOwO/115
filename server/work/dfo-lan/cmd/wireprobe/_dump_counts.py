import json

frames = json.load(open(r'D:\115us\analysis-tools\output\ispins_switch_frames.json', encoding='utf-8'))
print('total', len(frames))
for i, f in enumerate(frames):
    if f.get('op') in (643, 528, 1432, 1326, 2152, 1280, 1220):
        ph = f.get('plain_hex') or ''
        print('frame %4d %s op=%-4d %-40s len=%d' % (i, f['dir'], f['op'], f['op_name'], f['len']))
        print('   ', ph[:120])

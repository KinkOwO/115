import json, sys

p = r'D:\115us\analysis-tools\output\ispins_switch_frames.json'
d = json.load(open(p, encoding='utf-8'))
base = [f for f in d if f['id'] == 894][0]['ts']
for f in sorted(d, key=lambda x: x['id']):
    if 880 <= f['id'] <= 1310 and f.get('op'):
        print("%4d %+8.2fs %s op=%-5d %-44s len=%d plain=%s" % (
            f['id'], (f['ts'] - base) / 1000.0, f['dir'], f['op'],
            str(f.get('op_name'))[:44], f['len'], str(f.get('plain_hex', ''))[:40]))

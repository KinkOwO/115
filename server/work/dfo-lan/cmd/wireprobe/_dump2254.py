import json

frames = [json.loads(l) for l in open(r'D:\115us\实时抓包\captures\20261003-041758\frames.jsonl', encoding='utf-8')]
town = None
standby = None
for i, f in enumerate(frames):
    if f.get('op') == 2254 and f.get('dir') == 's2c':
        if town is None:
            town = (i, f.get('plain_hex') or '')
        else:
            standby = (i, f.get('plain_hex') or '')

official = json.load(open(r'D:\115us\analysis-tools\output\ispins_switch_frames.json', encoding='utf-8'))
# official standby entry N2254 = frame 997 per memory; verify
off = None
for i, f in enumerate(official):
    if f.get('op') == 2254 and f.get('dir') == 's2c':
        print('official 2254 at frame', i, 'len', f['len'])
        if off is None:
            off = (i, f.get('plain_hex') or '')

for label, rec in (('private town', town), ('private standby', standby), ('official first', off)):
    if rec is None:
        print(label, 'missing')
        continue
    i, h = rec
    b = bytes.fromhex(h)
    print('=== %s (frame %d, %d bytes)' % (label, i, len(b)))
    # print nonzero bytes with offsets
    nz = [(o, v) for o, v in enumerate(b) if v != 0]
    print('nonzero:', ' '.join('%02x:%02x' % (o, v) for o, v in nz))

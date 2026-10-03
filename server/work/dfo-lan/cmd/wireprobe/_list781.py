import json, sys

path = sys.argv[1] if len(sys.argv) > 1 else r'D:\115us\实时抓包\captures\20261003-041758\frames.jsonl'
ops = {637, 706, 781, 782, 537, 1336, 2254, 2255, 2256, 108, 1198, 708}
frames = [json.loads(l) for l in open(path, encoding='utf-8')]
print('total', len(frames))
for i, f in enumerate(frames):
    if f.get('dir') == 's2c' and f.get('op') in ops:
        ph = f.get('plain_hex') or ''
        print(f"idx {i} op={f['op']} len={f.get('len')} {ph[:100]}")

import json

p = r'D:\115us\实时抓包\captures\20261003-032346\frames.jsonl'
frames = []
for line in open(p, encoding='utf-8'):
    frames.append(json.loads(line))

sel = [f for f in frames if str(f.get('remote', '')).endswith(':54542') or str(f.get('local', '')).endswith(':54542')]
base = sel[0]['ts']
print('==== town conn 54542 : %d frames ====' % len(sel))
for f in sel:
    op = f.get('op')
    # only print non-heartbeat ops plus first/last heartbeat
    print("%+9.2fs %s op=%-5d %-46s len=%d plain=%s" % (
        (f['ts'] - base) / 1000.0, f['dir'], op if op is not None else -1,
        str(f.get('op_name'))[:46], f['len'], str(f.get('plain_hex', ''))[:44]))

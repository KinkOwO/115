# 带时间戳检查 781 帧序列 + 会话尾部活动
import json, datetime

frames = [json.loads(l) for l in open(r'D:\115us\实时抓包\captures\20261003-041758\frames.jsonl', encoding='utf-8')]
print("total frames:", len(frames))
for i, f in enumerate(frames):
    if f.get('op') == 781 and f.get('dir') == 's2c' and f.get('plain_hex'):
        b = bytes.fromhex(f['plain_hex'])
        if len(b) == 272:
            ts = f.get('ts')
            t = datetime.datetime.fromtimestamp(ts/1000).strftime('%H:%M:%S') if ts else '?'
            print(f"idx {i} ts={t} state={b[4:8].hex()} 0xff={b[0xff]:02x}")
# 最后 10 帧的时间与 op
print("--- last 10 frames ---")
for f in frames[-10:]:
    ts = f.get('ts')
    t = datetime.datetime.fromtimestamp(ts/1000).strftime('%H:%M:%S') if ts else '?'
    print(t, f.get('dir'), f.get('op'), f.get('len'))

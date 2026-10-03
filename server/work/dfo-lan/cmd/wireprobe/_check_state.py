import json

# private capture 041758: extract standby 781 full body, 706 full body
frames = [json.loads(l) for l in open(r'D:\115us\实时抓包\captures\20261003-041758\frames.jsonl', encoding='utf-8')]
for i, f in enumerate(frames):
    if f.get('dir') == 's2c' and f.get('op') == 781:
        b = bytes.fromhex(f['plain_hex'])
        if len(b) >= 272:
            print(f"idx {i} 781 len={len(b)} state@4={b[4:8].hex()} 0xff={b[0xff]:02x} tail={b[-16:].hex()}")
    if f.get('dir') == 's2c' and f.get('op') == 2254:
        b = bytes.fromhex(f['plain_hex'])
        if len(b) >= 32:
            print(f"idx {i} 2254 len={len(b)} flags0_16={b[0:16].hex()} off21={b[21]:02x}")

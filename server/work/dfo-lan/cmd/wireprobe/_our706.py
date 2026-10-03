import json

frames = [json.loads(l) for l in open(r'D:\115us\实时抓包\captures\20261003-041758\frames.jsonl', encoding='utf-8')]
for i, f in enumerate(frames):
    if f.get('dir') == 's2c' and f.get('op') == 706:
        b = bytes.fromhex(f['plain_hex'])
        print(f"idx {i} 706 len={len(b)}")
        print(f"  0x2de={b[0x2de]:02x}")
        print(f"  tail 0x578-0x598: {b[0x578:0x598].hex()}")
        print(f"  off1409={b[0x581]:02x} off1414={b[0x586]:02x} off1416={b[0x588]:02x} off1420={b[0x58c]:02x} off1424={b[0x590]:02x}")
        break

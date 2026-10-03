import json

frames = [json.loads(l) for l in open(r'D:\115us\实时抓包\captures\20261003-041758\frames.jsonl', encoding='utf-8')]
login = standby = None
for i, f in enumerate(frames):
    if f.get('dir') == 's2c' and f.get('op') == 2254:
        b = bytes.fromhex(f['plain_hex'])
        if b[:2] == b'\x00\x00' and login is None:
            login = b
        elif b[:2] == b'\x01\x00' and standby is None:
            standby = b
        if login and standby:
            break

print('login len', len(login), 'standby len', len(standby))
diffs = [i for i in range(272) if login[i] != standby[i]]
print('diff offsets:', diffs)
for i in diffs:
    print(f"  off {i} (0x{i:x}): login={login[i]:02x} standby={standby[i]:02x}")
print('login tail:', login[-24:].hex())
print('standby tail:', standby[-24:].hex())

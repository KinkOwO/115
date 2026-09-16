"""Pure native routines only; regenerate independent slot 3/7 vectors."""
import json
from pathlib import Path
from native_cipher_oracle import Oracle

o = Oracle()
rows = []
for seed in (0, 1, 71):
    key = bytes((i + seed) & 255 for i in range(32))
    plain = bytes((i * 7 + seed) & 255 for i in range(32))
    cipher = o.cipher(key, plain, 0x146D99BA0, 0x146D99B80, 0x146D99B60, 16, 16)
    rows.append(dict(algorithm='twofish', key=key.hex(), plain=plain.hex(), cipher=cipher.hex(), encrypt='146d99b80', decrypt='146d99b60'))
for seed in (0, 1, 71):
    key = bytes((i + seed) & 255 for i in range(56))
    plain = bytes((i * 7 + seed) & 255 for i in range(24))
    o.u.mem_write(0x210000, key)
    o.u.mem_write(0x260000, b'\0' * 8)
    o.call(0x146D90E30, 0x220000, 0x210000, 56, 0x260000)
    o.u.mem_write(0x230000, plain)
    o.call(0x146D91BF0, 0x220000, 0x230000, 0x240000, len(plain), 0)
    cipher = bytes(o.u.mem_read(0x240000, len(plain)))
    o.call(0x146D91450, 0x220000, 0x240000, 0x250000, len(plain), 0)
    assert bytes(o.u.mem_read(0x250000, len(plain))) == plain
    rows.append(dict(algorithm='blowfish', key=key.hex(), plain=plain.hex(), cipher=cipher.hex(), encrypt='146d91bf0', decrypt='146d91450'))
out = Path(__file__).parent.parent / 'dfo-lan/internal/game/wire/testdata/native_world_ciphers.json'
out.write_text(json.dumps(rows, indent=2), encoding='utf-8')
print(f'native world cipher vectors: {len(rows)}')

"""Execute only the exact-build slot 10 pure routines in private emulation."""
from native_cipher_oracle import Oracle
import json, pathlib, struct

out = pathlib.Path(__file__).parent.parent / 'dfo-lan/internal/game/wire/testdata/native_xor32_vectors.json'
o = Oracle()
obj, keyp, src, dst = 0x220000, 0x210000, 0x230000, 0x240000
o.u.mem_write(obj, struct.pack('<Q', 0x14b1742b0) + bytes(16))
assert o.call(0x14035ab20, obj) == 4
rows = []
for seed in (0, 1, 71, 255):
    key = bytes((i + seed) & 255 for i in range(8))
    o.u.mem_write(keyp, key)
    assert o.call(0x146d8dd00, obj, keyp, 8) == 0x6fffffff
    for size in (4, 20, 64, 68):
        plain = bytes((i * 7 + seed) & 255 for i in range(size))
        o.u.mem_write(src, plain)
        assert o.call(0x146d8dbb0, obj, src, size, dst, size) == 0x6fffffff
        encrypted = bytes(o.u.mem_read(dst, size))
        assert o.call(0x146d8da70, obj, dst, size) == 0x6fffffff
        assert bytes(o.u.mem_read(dst, size)) == plain
        rows.append(dict(algorithm='dfo_xor32', key=key.hex(), plain=plain.hex(), cipher=encrypted.hex(),
                         setup='0x146d8dd00', transform='0x146d8dbb0', inverse='0x146d8da70'))
out.write_text(json.dumps(rows, indent=2))
print(f'native slot 10 vectors and roundtrips: {len(rows)}; block alignment: 4')

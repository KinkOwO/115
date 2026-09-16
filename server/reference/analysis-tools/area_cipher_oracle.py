"""Exact-build slot 9 constants and native encrypt/decrypt fixtures.

Only pure cipher code is emulated. Wrapper 146d8b240 selects a 16-byte key
and 12 rounds; encrypt 146d8b150 calls 146d90df0, decrypt calls 146d90dd0.
"""
import json
import struct
from pathlib import Path
from native_cipher_oracle import Oracle

o = Oracle()
root = Path(__file__).parent.parent / 'dfo-lan/internal/game/wire'
tables = []
for i, address in enumerate([0x14b1745f0, 0x14b1749f0, 0x14b174df0, 0x14b1751f0, 0x14b1755f0, 0x14b1759f0, 0x14b175df0]):
    count = 12 if i == 6 else 256
    values = struct.unpack('<' + 'I' * count, o.pe.get_data(address-o.base, count*4))
    tables.append('var areaCipherT' + str(i) + ' = [...]uint32{\n' + '\n'.join('    '+', '.join(f'0x{v:08x}' for v in values[j:j+8])+',' for j in range(0,count,8)) + '\n}\n')
(root/'area_cipher_tables.go').write_text('// Generated from the supplied EXE by area_cipher_oracle.py.\npackage wire\n\n'+'\n'.join(tables), encoding='utf-8')
rows = []
for seed in (0, 1, 71, 238):
    key = bytes((i+seed)&255 for i in range(16))
    plain = bytes((i*7+seed)&255 for i in range(48))
    cipher = o.cipher(key, plain, 0x146d90e10, 0x146d90df0, 0x146d90dd0, 12, 16)
    rows.append(dict(key=key.hex(), plain=plain.hex(), cipher=cipher.hex(), setup='146d90e10', encrypt='146d90df0', decrypt='146d90dd0'))
(root/'testdata/native_area_cipher.json').write_text(json.dumps(rows,indent=2), encoding='utf-8')
print('slot 9 native vectors:',len(rows))

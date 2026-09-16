"""Recover slot 6 constants and independent vectors from pure native routines."""
import json, struct
from pathlib import Path
from native_cipher_oracle import Oracle

o = Oracle()
root = Path(__file__).parent.parent / 'dfo-lan/internal/game/wire'
constants = []
for name, address, count in [('kasumiC', 0x14b17b940, 8), ('kasumiS7', 0x14b17af40, 128), ('kasumiS9', 0x14b17b140, 512)]:
    values = struct.unpack('<' + 'I' * count, o.pe.get_data(address-o.base, count*4))
    constants.append('var ' + name + ' = [...]uint16{\n' + '\n'.join('    '+', '.join(str(v) for v in values[i:i+16])+',' for i in range(0,count,16)) + '\n}\n')
(root/'kasumi_tables.go').write_text('// Exact-build slot 6 substitution and key constants; quest_cipher_oracle.py.\npackage wire\n\n' + '\n'.join(constants), encoding='utf-8')
rows = []
for seed in (0, 1, 71):
    key = bytes((i+seed)&255 for i in range(16))
    plain = bytes((i*7+seed)&255 for i in range(24))
    cipher = o.cipher(key, plain, 0x146d93be0, 0x146d93a80, 0x146d93920, 8, 8)
    rows.append(dict(algorithm='kasumi', key=key.hex(), plain=plain.hex(), cipher=cipher.hex(), setup='146d93be0', encrypt='146d93a80', decrypt='146d93920'))
(root/'testdata/native_quest_cipher.json').write_text(json.dumps(rows,indent=2), encoding='utf-8')
print('slot 6 native vectors:', len(rows))

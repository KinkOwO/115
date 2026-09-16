"""Exact-build slot 13 vectors; pure native routine emulation only."""
from native_cipher_oracle import Oracle
import json,pathlib
p=pathlib.Path(__file__).parent
o=Oracle();rows=[]
for seed in (0,1,71,255):
 key=bytes((i+seed)&255 for i in range(40))
 plain=bytes((i*7+seed)&255 for i in range(16))
 encrypted=o.cipher(key,plain,0x146d96670,0x146d964c0,0x146d96330,128,8)
 rows.append(dict(algorithm='dfo_multi2',key=key.hex(),plain=plain.hex(),cipher=encrypted.hex(),
                  setup='0x146d96670',encrypt='0x146d964c0',decrypt='0x146d96330'))
(p.parent/'dfo-lan/internal/game/wire/testdata/native_multi2_vectors.json').write_text(json.dumps(rows,indent=2))
print('native slot 13 roundtrips:',len(rows))

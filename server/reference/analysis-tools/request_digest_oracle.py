"""Original current client digest routine, private memory only."""
from native_cipher_oracle import Oracle
import json,pathlib,hashlib
p=pathlib.Path(__file__).parent.parent/'dfo-lan'
samples=[bytes.fromhex('000100ffffffff'),bytes.fromhex('000200ffffffff'),bytes.fromhex(json.loads((p/'internal/game/protocol/testdata/live27_play_result.json').read_text())['payload_hex'])[:113]]
rows=[]
for raw in samples:
 o=Oracle();o.u.mem_write(0x220000,raw)
 assert o.call(0x1401be160,0x220000,len(raw),0x230000)&255==1
 actual=bytes(o.u.mem_read(0x230000,4));d=hashlib.md5(raw).digest()
 expected=bytes([d[13]^d[8]^d[10]^d[0]^0x81,d[12]^d[5]^d[9]^d[1]^0x78,d[14]^d[4]^d[6]^d[2]^0x1a,d[15]^d[7]^d[11]^d[3]^0xbf])
 assert actual==expected
 rows.append({'body':raw.hex(),'digest':actual.hex()})
(p/'internal/game/protocol/testdata/native_request_digests.json').write_text(json.dumps(rows,indent=2),encoding='utf-8')
print('NATIVE_DIGEST_PASS',len(rows))

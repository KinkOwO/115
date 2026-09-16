"""Decode bounded owned-loopback CMD39 evidence with original pure cipher."""
from native_cipher_oracle import Oracle
from pathlib import Path
import json,sys
p=Path(__file__).parent.parent/'dfo-lan/runtime'/sys.argv[1]
keys=bytes(i%127+1 for i in range(334));start=sum([16,16,60,32,16,10,16,56,16,16,8]);key=keys[start:start+16]
o=Oracle();o.u.mem_write(0x210000,key);assert o.call(0x146d945d0,0x210000,16,8,0x220000)==0
rows=[]
for line in (p/'events.jsonl').read_text().splitlines():
 r=json.loads(line)
 if r.get('kind')!='client_frame' or r.get('id')!=39:continue
 raw=bytes.fromhex(r['hex']);plain=bytearray()
 for off in range(13,len(raw),8):
  o.u.mem_write(0x230000,raw[off:off+8]);o.call(0x146d945a0,0x230000,0x240000,0x220000);plain+=bytes(o.u.mem_read(0x240000,8))
 crc=0xffffffff
 for b in raw[11:13]+plain:
  crc^=b
  for _ in range(8):crc=(crc>>1)^(0x4db89129 if crc&1 else 0)
 crc^=0xffffffff;check=(crc^(crc>>8)^(crc>>16)^(crc>>24)^0x18)&255
 rows.append(dict(time=r['time'],plain_hex=plain.hex(),checksum_ok=check==raw[7]))
 if len(rows)>=3:break
(p/'combat_decoded.json').write_text(json.dumps(rows,indent=2))
print(json.dumps(rows,indent=2))

"""Current native64-bit PVF combination, including high-level thresholds."""
from native_cipher_oracle import Oracle
import pathlib,json,struct
p=pathlib.Path(__file__).parent.parent/'dfo-lan'
c=json.loads((p/'runtime/progression_rules/00-exptable.tbl.tokens.json').read_text(encoding='utf-8'))
o=Oracle();rows=[];i=0
while i<len(c):
 if c[i]['type']==0:v=c[i]['value'];i+=1
 else:
  assert c[i]['type']==c[i+1]['type']==9
  high,low=c[i]['value']&0xffffffff,c[i+1]['value']&0xffffffff
  assert o.call(0x147c99070,0,high,low,0x220000)&255==1
  v=struct.unpack('<Q',o.u.mem_read(0x220000,8))[0]
  assert v==(high<<32|low);i+=2
 rows.append(v)
assert len(rows)==150 and all(a<b for a,b in zip(rows,rows[1:]))
dest=p/'internal/catalog/testdata/native_experience_thresholds.json'
dest.write_text(json.dumps(dict(thresholds=rows,scope='type9 pairs combined by original147c99070; type0 direct source scalars'),indent=2),encoding='utf-8')
print('native experience thresholds',len(rows),'last',rows[-1])

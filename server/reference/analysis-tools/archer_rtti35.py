"""Read-only current character virtual skill factory lookup."""
from native_cipher_oracle import Oracle
import re,struct,json
from pathlib import Path
o=Oracle(); data=o.pe.__data__; rows=[]
print('known_character_vtable',hex(0x14ad21370), 'skill_getter',hex(struct.unpack('<Q',o.pe.get_data(0x14ad21370-o.base+0x2048,8))[0]))
for m in re.finditer(rb'\.\?AV[^\x00]{1,150}\x00', data):
 name=m.group()[:-1].decode('ascii','replace')
 if 'archer' not in name.lower() and 'swordman' not in name.lower(): continue
 typeva=o.base+o.pe.get_rva_from_offset(m.start()-16)
 row={'name':name,'type':hex(typeva),'vtables':[]}
 for q in re.finditer(re.escape(struct.pack('<I',typeva-o.base)), data):
  offset=q.start()-12
  if offset<0:continue
  col=struct.unpack_from('<6I',data,offset)
  if col[0]!=1:continue
  colva=o.base+o.pe.get_rva_from_offset(offset)
  if col[5]+o.base!=colva:continue
  for ptr in re.finditer(re.escape(struct.pack('<Q',colva)),data):
   vt=o.base+o.pe.get_rva_from_offset(ptr.start())+8
   row['vtables'].append({'vtable':hex(vt),'offset':col[1], 'get_skill':hex(struct.unpack('<Q',o.pe.get_data(vt-o.base+0x2048,8))[0])})
 rows.append(row)
out=Path(__file__).with_suffix('.json');out.write_text(json.dumps(rows,indent=2));print(json.dumps(rows,indent=2))

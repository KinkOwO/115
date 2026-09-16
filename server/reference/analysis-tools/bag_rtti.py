"""Resolve inventory RTTI/vtable from the current binary, read-only."""
import struct,re
from native_cipher_oracle import Oracle
o=Oracle();data=o.pe.__data__
# Target RTTI is the exact dynamic-cast type referenced by ITEM_LIST kind0.
va=0x1452d5d1c;code=o.pe.get_data(va-o.base,7)
typeva=va+7+struct.unpack_from('<i',code,3)[0]
print('type',hex(typeva),o.pe.get_data(typeva-o.base+16,100).split(b'\0')[0])
needle=struct.pack('<I',typeva-o.base)
for m in re.finditer(re.escape(needle),data):
 off=m.start()-12
 if off<0:continue
 cols=struct.unpack_from('<6I',data,off)
 if cols[0]!=1:continue
 try: colva=o.base+o.pe.get_rva_from_offset(off)
 except:continue
 if cols[5]+o.base!=colva:continue
 for ptr in re.finditer(re.escape(struct.pack('<Q',colva)),data):
  vt=o.base+o.pe.get_rva_from_offset(ptr.start())+8
  print('vtable',hex(vt),'offset',cols[1],'open',hex(struct.unpack('<Q',o.pe.get_data(vt-o.base+0x28,8))[0]))

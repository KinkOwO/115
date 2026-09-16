"""Current NOTI38 single-item reader; native field copies, stubbed item APIs."""
from dungeon_map_oracle import PacketOracle
from unicorn.x86_const import *
import struct,json,pathlib

class DropItemOracle(PacketOracle):
 def __init__(self,p):
  super().__init__(p,0x1452bbd58,0x1452bc316)
  self.u.reg_write(UC_X86_REG_RBP,0x280000)
  self.u.reg_write(UC_X86_REG_R13,0)
  self.u.mem_write(0x27ff80,struct.pack('<H',1))

if __name__=='__main__':
 dest=pathlib.Path(__file__).parent.parent/'dfo-lan/internal/game/protocol/testdata'
 rows=[]
 for template,amount in [(0,34),(3327,2)]:
  raw=bytearray(181);struct.pack_into('<HII',raw,0,120,template,amount)
  row=struct.pack('<I',12345)+raw+struct.pack('<IHH',0,65535,3)
  o=DropItemOracle(row);r=o.parse()
  r['native_item_fields']=dict(scene=struct.unpack('<I',o.u.mem_read(0x2800fc,4))[0],template=struct.unpack('<I',o.u.mem_read(0x2800f4,4))[0],owner=struct.unpack('<H',o.u.mem_read(0x280108,2))[0])
  assert r['consumed']==193 and r['native_item_fields']==dict(scene=12345,template=template,owner=3)
  try:DropItemOracle(row[:-1]).parse()
  except ValueError as e:r['truncation_failure']=str(e)
  else:raise AssertionError('short item accepted')
  r['scope']='single current NOTI38 row, native181-byte copy and trailing scalar/owner reads; item predicates stubbed to ordinary branch, no drop or inventory created'
  rows.append(r)
 (dest/'native_drop_items.json').write_text(json.dumps(rows,indent=2),encoding='utf-8')
 print('native drop rows',[r['native_item_fields'] for r in rows])

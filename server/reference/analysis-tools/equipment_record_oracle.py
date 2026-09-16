from inventory_restore_oracle import InventoryOracle
from unicorn.x86_const import *
import struct,json,pathlib
class EquipmentOracle(InventoryOracle):
 def __init__(self,p):super().__init__(p);self.items=[]
 def step(self,u,a,n,_):
  ins=next(self.md.disasm(bytes(u.mem_read(a,n)),a))
  if ins.mnemonic=='call' and ins.op_str=='0x145770280':
   p=bytes(u.mem_read(u.reg_read(UC_X86_REG_RCX),181));self.items.append({'slot':struct.unpack_from('<H',p)[0],'template':struct.unpack_from('<I',p,2)[0],'data':struct.unpack_from('<I',p,6)[0],'durability':struct.unpack_from('<H',p,11)[0]})
  super().step(u,a,n,_)
if __name__=='__main__':
 row=bytearray(181);struct.pack_into('<HII',row,0,9,20002,0);struct.pack_into('<H',row,11,25)
 data=struct.pack('<BHH',0,0,1)+row
 o=EquipmentOracle(data);r=o.parse();assert o.items==[{'slot':9,'template':20002,'data':0,'durability':25}]
 r['items']=o.items;r['scope']='original native list0 packet read and181-byte equipment record transfer; durability25 is a field sentinel; item factory/UI stubbed'
 p=pathlib.Path(__file__).parent.parent/'dfo-lan/internal/game/protocol/testdata/native_equipment_record.json';p.write_text(json.dumps(r,indent=2))
 print('NATIVE_EQUIPMENT_RECORD_PASS',o.items)

"""Original current NOTI13 ordinary inventory cursor, isolated emulation."""
from dungeon_map_oracle import PacketOracle
from unicorn.x86_const import *
import struct,json,pathlib

class InventoryOracle(PacketOracle):
 def __init__(self,p):super().__init__(p,0x1452d5a80,0x1452d745a)
 def step(self,u,addr,size,_):
  ins=next(self.md.disasm(bytes(u.mem_read(addr,size)),addr))
  if ins.mnemonic=='call' and ins.op_str in {'0x145f0ba60','0x145efafb0','0x145a0e8c0'}:
   u.reg_write(UC_X86_REG_RAX,0x250000);u.reg_write(UC_X86_REG_RIP,addr+size);return
  super().step(u,addr,size,_)

if __name__=='__main__':
 dest=pathlib.Path(__file__).parent.parent/'dfo-lan/internal/game/protocol/testdata'
 for count in [0,1,2]:
  data=bytearray(struct.pack('<BHH',0,0,count))
  for i in range(count):
   raw=bytearray(181);struct.pack_into('<HII',raw,0,0 if i==0 else 65,0 if i==0 else 3327,123)
   data+=raw
  o=InventoryOracle(bytes(data))
  try:r=o.parse()
  except Exception:
   print('cursor',o.pos,'rip',hex(o.u.reg_read(UC_X86_REG_RIP)),'reads',o.reads[-5:]);raise
  r['scope']='current native NOTI13 list0 cursor; ordinary item, game and UI lookups stubbed'
  (dest/f'native_inventory_restore_{count}.json').write_text(json.dumps(r,indent=2),encoding='utf-8')
  print('inventory restore',count,r['consumed'])

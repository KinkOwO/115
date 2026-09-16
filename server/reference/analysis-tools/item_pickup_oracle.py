"""Current ordinary/gold item updates and pickup tails, private emulation."""
from dungeon_map_oracle import PacketOracle
from unicorn.x86_const import *
import struct,json,pathlib

class ItemUpdateOracle(PacketOracle):
 def __init__(self,payload):super().__init__(payload,0x1452e9810,0x1452eb82f)
 def step(self,u,addr,size,_):
  ins=next(self.md.disasm(bytes(u.mem_read(addr,size)),addr))
  if ins.mnemonic=='call' and ins.op_str in {'0x145f0ba60','0x145efafb0','0x145a0e8c0'}:
   u.reg_write(UC_X86_REG_RAX,0x250000);u.reg_write(UC_X86_REG_RIP,addr+size);return
  super().step(u,addr,size,_)

class PickupTailOracle(PacketOracle):
 def __init__(self,payload,gold):
  super().__init__(payload,0x1452d287f if gold else 0x1452d22f3,0x1452d2d4a if gold else 0x1452d2349)
  self.u.reg_write(UC_X86_REG_RBP,0x280000)
  self.u.reg_write(UC_X86_REG_R15,8)
  self.u.reg_write(UC_X86_REG_R14,0)

if __name__=='__main__':
 dest=pathlib.Path(__file__).parent.parent/'dfo-lan/internal/game/protocol/testdata'
 for template,slot in [(0,0),(3327,57)]:
  raw=bytearray(181);struct.pack_into('<HII',raw,0,slot,template,123)
  payload=struct.pack('<BH',0,1)+raw
  o=ItemUpdateOracle(payload)
  try:r=o.parse()
  except Exception:
   print('update cursor',o.pos,'rip',hex(o.u.reg_read(UC_X86_REG_RIP)),'reads',o.reads[-5:]);raise
  r['scope']='native ordinary list0 raw181 cursor; game/item lookups stubbed; no item granted'
  (dest/f'native_item_update_{template}.json').write_text(json.dumps(r,indent=2),encoding='utf-8')
  print('item update',template,r['consumed'])
 for gold in [False,True]:
  tail=bytes(8)+struct.pack('<HHB',3,0 if gold else 57,int(gold))
  if gold:tail=bytes(40)
  o=PickupTailOracle(tail,gold);r=o.parse()
  r['scope']='native current pickup tail after separately verified u32 object/u16 actor prefix; gold party flags neutral, money restored by NOTI14'
  (dest/f'native_pickup_tail_{int(gold)}.json').write_text(json.dumps(r,indent=2),encoding='utf-8')
  print('pickup tail gold=',gold,'bytes',r['consumed'])

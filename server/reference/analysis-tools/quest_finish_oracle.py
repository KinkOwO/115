"""Ordinary no-item quest completion reader, private emulator only."""
from dungeon_map_oracle import PacketOracle
from unicorn.x86_const import *
import struct,json,pathlib

class QuestFinishOracle(PacketOracle):
 def __init__(self,payload):
  super().__init__(payload,0x14527aa60,0x14527d307)
  self.u.reg_write(UC_X86_REG_RDX,1) # dispatcher already consumed success byte
  self.heap=0x260000
 def step(self,u,addr,size,_):
  ins=next(self.md.disasm(bytes(u.mem_read(addr,size)),addr))
  if ins.mnemonic=='call' and ins.op_str=='0x146e8ba20':
   ret=self.heap;self.heap+=(u.reg_read(UC_X86_REG_RCX)+31)&~31
   if self.heap>0x2a0000:raise ValueError('quest mock heap bound')
   u.reg_write(UC_X86_REG_RAX,ret);u.reg_write(UC_X86_REG_RIP,addr+size);return
  super().step(u,addr,size,_)

if __name__=='__main__':
 payload=struct.pack('<HBI3B',3145,0,1200,0,0,0)
 o=QuestFinishOracle(payload)
 try:r=o.parse()
 except Exception:
  print('quest cursor',o.pos,'rip',hex(o.u.reg_read(UC_X86_REG_RIP)),'last',o.reads[-5:]);raise
 r['scope']='current success-body no-item path, quest/game/UI calls stubbed; dispatcher success byte excluded; no real quest completed'
 dest=pathlib.Path(__file__).parent.parent/'dfo-lan/internal/game/protocol/testdata/native_quest_finish_empty.json'
 dest.write_text(json.dumps(r,indent=2),encoding='utf-8')
 print('quest success consumed',r['consumed'])

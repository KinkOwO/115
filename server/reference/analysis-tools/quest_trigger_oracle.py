"""Original NOTI291 reader and trigger-call arguments in private emulation."""
from dungeon_map_oracle import PacketOracle
from unicorn.x86_const import *
import struct,json,pathlib

class TriggerOracle(PacketOracle):
 def __init__(self,p):
  super().__init__(p,0x1452de670,0x1452de803);self.triggers=[];self.reset=False
 def step(self,u,addr,size,_):
  ins=next(self.md.disasm(bytes(u.mem_read(addr,size)),addr))
  if ins.mnemonic=='call':
   if ins.op_str=='0x144f426c0':self.reset=True
   if ins.op_str=='0x144f38b60':self.triggers.append([u.reg_read(UC_X86_REG_RDX),u.reg_read(UC_X86_REG_R8)])
  super().step(u,addr,size,_)

rows=[]
for progress in (1,0):
 payload=struct.pack('<HHIII',1,3145,progress,0,0)
 o=TriggerOracle(payload);r=o.parse()
 assert o.reset and o.triggers==[[3145,progress]]
 r.update(reset=o.reset,triggers=o.triggers);rows.append(r)
 try:TriggerOracle(payload[:-1]).parse()
 except ValueError:pass
 else:raise AssertionError('truncated packet accepted')
dest=pathlib.Path(__file__).parent.parent/'dfo-lan/internal/game/protocol/testdata/native_quest_triggers.json'
dest.write_text(json.dumps(rows,indent=2),encoding='utf-8')
print('Native quest trigger restore verified: pending1 / ready0, truncation rejected')

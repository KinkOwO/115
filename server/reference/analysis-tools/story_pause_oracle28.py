from dungeon_map_oracle import PacketOracle
from unicorn.x86_const import *
import struct,pathlib,json
class StoryOracle(PacketOracle):
 def __init__(self,p):super().__init__(p,0x1452ac690,0x1452ac83a);self.effects=[]
 def step(self,u,a,n,_):
  if a in (0x1452ac7ca,0x1452ac83a):self.done=True;u.emu_stop();return
  ins=next(self.md.disasm(bytes(u.mem_read(a,n)),a))
  if ins.mnemonic=='call':
   if ins.op_str=='0x1459a90f0':u.reg_write(UC_X86_REG_RAX,3);u.reg_write(UC_X86_REG_RIP,a+n);return
   if ins.op_str=='qword ptr [rax + 0x70]':u.reg_write(UC_X86_REG_RAX,0x250000);u.reg_write(UC_X86_REG_RIP,a+n);return
   if ins.op_str=='0x145ea4240':self.effects.append(u.reg_read(UC_X86_REG_RDX)&255)
  super().step(u,a,n,_)
if __name__=='__main__':
 rows=[]
 for state in (0,1):
  for kind in (0,1,2,3):
   o=StoryOracle(struct.pack('<HBB',3,state,kind));r=o.parse();r.update(state=state,kind=kind,pause_values=o.effects)
   expected=[] if state==0 and kind==3 else [1-state]
   assert r['consumed']==4 and o.effects==expected
   rows.append(r)
 p=pathlib.Path(__file__).parent.parent/'dfo-lan/internal/game/protocol/testdata/native_story_pause.json';p.write_text(json.dumps(rows,indent=2));print('NATIVE_STORY_PAUSE_PASS',len(rows))

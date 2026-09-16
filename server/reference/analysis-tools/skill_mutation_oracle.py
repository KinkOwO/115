from dungeon_map_oracle import PacketOracle
from unicorn.x86_const import *
import struct,json,pathlib
class SkillOracle(PacketOracle):
 def __init__(self,p,kind):
  super().__init__(p,0x14526ac70 if kind==29 else 0x14526d990,0x14526c411 if kind==29 else 0x14526dadb);self.effects=[]
 def call(self,va,*args):return super().call(va,0,1,0)
 def step(self,u,a,n,_):
  ins=next(self.md.disasm(bytes(u.mem_read(a,n)),a))
  if ins.mnemonic=='call' and ins.op_str.startswith('0x'):
   t=int(ins.op_str,16)
   if t in {0x145efafb0,0x145a0f140}:
    u.reg_write(UC_X86_REG_RAX,0x250000);u.reg_write(UC_X86_REG_RIP,a+n);return
   if t in {0x145ef9430,0x145ef9c00,0x145ef9390,0x145ef1fd0,0x1421a9320}:
    self.effects.append({'call':hex(t),'c':u.reg_read(UC_X86_REG_RCX)&0xffffffff,'d':u.reg_read(UC_X86_REG_RDX)&0xffffffff,'r8':u.reg_read(UC_X86_REG_R8)&0xffffffff})
    if t==0x145ef1fd0:
     u.reg_write(UC_X86_REG_RAX,1);u.reg_write(UC_X86_REG_RIP,a+n);return
  super().step(u,a,n,_)
if __name__=='__main__':
 dest=pathlib.Path(__file__).parent.parent/'dfo-lan/internal/game/protocol/testdata'
 for kind,p in [(28,bytes([0,1,3])),(29,bytes([0])+struct.pack('<HHB',10,0,1)+bytes([14])+struct.pack('<H',46)+bytes([2,0,0,0,0]))]:
  o=SkillOracle(p,kind);r=o.parse();r['effects']=o.effects
  r['scope']='current native skill mutation cursor and setter arguments; UI/actor bodies stubbed'
  (dest/f'native_skill_mutation_{kind}.json').write_text(json.dumps(r,indent=2),encoding='utf-8');print(kind,r['consumed'],o.effects)

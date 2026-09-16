from skill_mutation_oracle import SkillOracle
from dungeon_map_oracle import PacketOracle
import pathlib,json
import struct
from unicorn.x86_const import *
class RefusalOracle(SkillOracle):
 def __init__(self,kind):
  super().__init__(b'',kind)
  self.kind=kind;self.rollbacks=[]
  self.u.mem_write(0x2500d0,struct.pack('<I',2))
  if kind==28:self.end=0x145ef929d
 def call(self,va,*args):return PacketOracle.call(self,va,0,0,4)
 def step(self,u,a,n,_):
  ins=next(self.md.disasm(bytes(u.mem_read(a,n)),a))
  if ins.mnemonic=='call' and ins.op_str=='0x141c39340':
   u.reg_write(UC_X86_REG_RAX,struct.unpack('<I',u.mem_read(u.reg_read(UC_X86_REG_RCX)+0xd0,4))[0]);u.reg_write(UC_X86_REG_RIP,a+n);return
  if ins.mnemonic=='call' and ins.op_str=='0x1421a86c0':self.rollbacks.append('clear_pending_purchase')
  super().step(u,a,n,_)
if __name__=='__main__':
 rows=[]
 for kind in (28,29):
  o=RefusalOracle(kind);r=o.parse();r.update(kind=kind,effects=o.effects)
  r['pending_counter']=struct.unpack('<I',o.u.mem_read(0x2500d0,4))[0];r['rollbacks']=o.rollbacks
  assert r['consumed']==0
  assert r['pending_counter']==1 if kind==28 else r['rollbacks']==['clear_pending_purchase']
  rows.append(r)
 p=pathlib.Path(__file__).parent.parent/'dfo-lan/internal/game/protocol/testdata/native_skill_refusals.json';p.write_text(json.dumps(rows,indent=2));print('NATIVE_SKILL_REFUSAL_ROLLBACK_PASS',2)

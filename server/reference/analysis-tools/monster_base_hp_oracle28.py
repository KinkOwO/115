"""Current table row reader: source token values, original field placement."""
from dungeon_map_oracle import PacketOracle
from unicorn.x86_const import *
import json,pathlib,struct
class BaseRowOracle(PacketOracle):
 def __init__(self,values):
  super().__init__(b'',0x14753a6f0,0x14753ab2a);self.values=values;self.index=1
  self.u.reg_write(UC_X86_REG_RBP,0x260000);self.u.reg_write(UC_X86_REG_RAX,values[0])
 def step(self,u,a,n,_):
  ins=next(self.md.disasm(bytes(u.mem_read(a,n)),a))
  if ins.mnemonic=='call' and ins.op_str=='0x14709dbf0':
   if self.index>=len(self.values):raise ValueError('source row overrun')
   u.reg_write(UC_X86_REG_RAX,self.values[self.index]&0xffffffff);self.index+=1;u.reg_write(UC_X86_REG_RIP,a+n);return
  super().step(u,a,n,_)
if __name__=='__main__':
 root=pathlib.Path(__file__).parent.parent/'dfo-lan';rows=[]
 for file,kind in [('monster_tables/00-commonmonsterbaseparameter.tbl.tokens.json','common'),('monster_rank_tables28/00-bossmonsterbaseparameter.tbl.tokens.json','boss')]:
  c=json.loads((root/'runtime'/file).read_text());values=[t['value'] for t in c]
  for level in [1,2,3,5,9]:
   v=values[(level-1)*25:level*25];o=BaseRowOracle(v);o.parse();assert o.index==25
   hp=struct.unpack('<I',o.u.mem_read(0x2600a8,4))[0];mp=struct.unpack('<I',o.u.mem_read(0x2600b0,4))[0]
   assert hp==v[5] and mp==v[6]
   rows.append({'table':kind,'level':level,'base_hp':hp,'base_mp':mp})
 out={'scope':'original table-reader field placement; token-reader and protected setters stubbed; HP field later copied by1477fffd1..147800037; final modifiers not measured','rows':rows}
 (root/'runtime/monster_base_hp28.json').write_text(json.dumps(out,indent=2));print('NATIVE_BASE_HP_FIELD_PASS',rows)

from dungeon_map_oracle import PacketOracle
from unicorn.x86_const import *
import struct,json,pathlib
class DeleteOracle(PacketOracle):
 def __init__(self,p,succ=True):
  super().__init__(p,0x1452519d0,0x1452521d5);self.success=succ;self.effects=[]
 def call(self,va,*args):return super().call(va,0,int(self.success),0 if self.success else 2)
 def step(self,u,a,n,_):
  ins=next(self.md.disasm(bytes(u.mem_read(a,n)),a))
  if ins.mnemonic=='call' and ins.op_str.startswith('0x'):
   t=int(ins.op_str,16)
   if t in {0x140235650,0x14023dfd0,0x14023dfc0,0x140228d00}:
    self.effects.append({'call':hex(t),'value':u.reg_read(UC_X86_REG_RDX)&65535})
  super().step(u,a,n,_)
if __name__=='__main__':
 dest=pathlib.Path(__file__).parent.parent/'dfo-lan/internal/game/protocol/testdata'
 for success in [True,False]:
  o=DeleteOracle(struct.pack('<BH',0,2) if success else b'',success);r=o.parse();r['effects']=o.effects
  assert [x['value'] for x in o.effects if x['call'] in ('0x14023dfd0','0x14023dfc0')]==[0,0]
  r['scope']='current native delete reader and roster removal/unlock arguments; game/UI bodies stubbed'
  (dest/f'native_delete_{int(success)}.json').write_text(json.dumps(r,indent=2),encoding='utf-8')
  print(success,r['consumed'],o.effects)

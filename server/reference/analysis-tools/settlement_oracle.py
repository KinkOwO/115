"""Current result readers; zero dynamic groups, original cursor and setters."""
from dungeon_map_oracle import PacketOracle
from unicorn.x86_const import *
import struct,json,pathlib

class SettlementOracle(PacketOracle):
 def __init__(self,payload,handler,end,discover=False):
  super().__init__(payload,handler,end);self.discover=discover;self.effects=[]
 def step(self,u,addr,size,_):
  ins=next(self.md.disasm(bytes(u.mem_read(addr,size)),addr))
  if ins.mnemonic=='call':
   target=int(ins.op_str,16) if ins.op_str.startswith('0x') else 0
   if target==0x148860800:
    u.reg_write(UC_X86_REG_RIP,addr+size);return
   if target==0x146ccbee0: # normal eligible dungeon result context
    u.reg_write(UC_X86_REG_RAX,1);u.reg_write(UC_X86_REG_RIP,addr+size);return
   if self.discover and target in {0x146ea09f0,0x146ea0be0,0x146ea1920,0x146ea0ba0}:
    n={0x146ea1920:2,0x146ea0ba0:4}.get(target,u.reg_read(UC_X86_REG_RDX))
    if n>4096:raise ValueError('unexpected read width')
    self.payload+=bytes(n)
   if target in {0x146a9a0b0,0x146a9aaf0}:
    self.effects.append(dict(setter=hex(target),value=u.reg_read(UC_X86_REG_RDX)&0xffffffff))
  super().step(u,addr,size,_)

if __name__=='__main__':
 dest=pathlib.Path(__file__).parent.parent/'dfo-lan/internal/game/protocol/testdata'
 for name,start,end in [('clear_reward',0x1452a6c20,0x1452a831f),('play_result',0x1452b1960,0x1452b1d0e)]:
  o=SettlementOracle(b'',start,end,True)
  try:r=o.parse()
  except Exception:
   print(name,'cursor',o.pos,'rip',hex(o.u.reg_read(UC_X86_REG_RIP)),'reads',o.reads[-6:]);raise
  r['scope']='native zero-dynamic-group cursor; gameplay/UI calls stubbed; research fixture only'
  (dest/f'native_{name}_empty.json').write_text(json.dumps(r,indent=2),encoding='utf-8')
  print(name,r['consumed'],'last reads',[(x['site'],x['offset'],x['size']) for x in o.reads[-16:]])
 for name,start,end in [('clear_reward',0x1452a6c20,0x1452a831f),('play_result',0x1452b1960,0x1452b1d0e)]:
  if name=='clear_reward':
   payload=bytearray(281)
   struct.pack_into('<II',payload,0,500,50);payload[167]=1
   struct.pack_into('<I',payload,169,1234)
   struct.pack_into('<I',payload,265,0xffffffff);struct.pack_into('<I',payload,277,0xffffffff)
  else:payload=struct.pack('<BIBBBBH5I',50,60000,0,50,1,1,3,60000,0,0,0,0)
  o=SettlementOracle(bytes(payload),start,end);r=o.parse();r['effects']=o.effects
  try:SettlementOracle(bytes(payload[:-1]),start,end).parse()
  except ValueError as e:r['truncation_failure']=str(e)
  else:raise AssertionError('short result packet accepted')
  r['scope']='current native full packet cursor with nonzero EXP/show-result or solo actor/time; game and UI methods stubbed'
  (dest/f'native_{name}_current.json').write_text(json.dumps(r,indent=2),encoding='utf-8')
  print(name,'nonzero consumed',r['consumed'],'effects',o.effects)

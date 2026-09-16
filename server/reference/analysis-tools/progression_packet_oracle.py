"""Private native cursor checks for clear/experience; no live game execution."""
from dungeon_map_oracle import PacketOracle
from unicorn.x86_const import *
import struct,json,pathlib

class ProgressOracle(PacketOracle):
 def __init__(self,payload,handler,end):
  super().__init__(payload,handler,end);self.heap=0x270000;self.effects=[]
  self.u.mem_write(0x260024,struct.pack('<I',1))
  # The EXP reader's current-character comparison; only identity is mocked.
  for addr,value in ((0x14ef2ca78,0x250030),(0x14ef2ca70,0x258000)):
   try:self.u.mem_read(addr,8)
   except Exception:self.missing(self.u,0,addr,8,0,None)
   self.u.mem_write(addr,struct.pack('<Q',value))
  self.u.mem_write(0x258008,struct.pack('<I',1))
 def step(self,u,addr,size,_):
  ins=next(self.md.disasm(bytes(u.mem_read(addr,size)),addr))
  if ins.mnemonic=='call':
   t=int(ins.op_str,16) if ins.op_str.startswith('0x') else 0
   if t==0x146d77f90:return # run actual two-u32/u64 adapter
   ret=None
   if t==0x146e8ba20:
    ret=self.heap;self.heap+=(u.reg_read(UC_X86_REG_RCX)+31)&~31
    if self.heap>=0x2e0000:raise ValueError('bounded mock allocation exhausted')
   elif t in (0x145efafb0,0x145f0ba60):ret=0x250000
   elif t==0x140176170:ret=0x260000
   if addr==0x1452cfad4:self.effects.append(dict(kind='actor_exp_level',experience=u.reg_read(UC_X86_REG_RDX),level=u.reg_read(UC_X86_REG_R8)))
   if ret is not None:
    u.reg_write(UC_X86_REG_RAX,ret);u.reg_write(UC_X86_REG_RIP,addr+size);return
  super().step(u,addr,size,_)

if __name__=='__main__':
 dest=pathlib.Path(__file__).parent.parent/'dfo-lan/internal/game/protocol/testdata'
 cases=[('clear_enabled',bytes(4),0x1452ae550,0x1452aee04),
        ('experience',bytes([1])+struct.pack('<Q',72)+bytes(73),0x1452ce4d0,0x1452cff55),
        ('experience_extended',bytes([2])+struct.pack('<QI4HI',0x100000048,0,30,40,2,3,11)+bytes(57),0x1452ce4d0,0x1452cff55)]
 for name,data,start,end in cases:
  o=ProgressOracle(data,start,end)
  try:r=o.parse()
  except Exception:
   print(name,'cursor',o.pos,'rip',hex(o.u.reg_read(UC_X86_REG_RIP)),'last reads',o.reads[-5:]);raise
  r['effects']=o.effects
  (dest/f'native_{name}.json').write_text(json.dumps(r,indent=2),encoding='utf-8')
  print(name,'consumed',r['consumed'],'effects',o.effects)

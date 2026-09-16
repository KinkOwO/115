"""Execute original NOTI27 reader control flow; stub game/UI side effects.

This establishes byte consumption for the solo, non-relay branch only.
It does not establish live UI acceptance or dungeon/quest completion.
"""
from native_cipher_oracle import Oracle
from unicorn.x86_const import *
import capstone, unicorn, json, pathlib

class GateOracle(Oracle):
 def __init__(self,payload):
  super().__init__()
  self.payload=payload;self.pos=0;self.reads=[];self.done=False
  self.md=capstone.Cs(capstone.CS_ARCH_X86,capstone.CS_MODE_64)
  self.u.mem_map(0,0x10000)
  self.u.hook_add(unicorn.UC_HOOK_CODE,self.step)
 def step(self,u,addr,size,_):
  if addr==0x145304298:
   self.done=True;u.emu_stop();return
  ins=next(self.md.disasm(bytes(u.mem_read(addr,size)),addr))
  if ins.mnemonic!='call':return
  target=int(ins.op_str,16) if ins.op_str.startswith('0x') else 0
  c=u.reg_read(UC_X86_REG_RCX);d=u.reg_read(UC_X86_REG_RDX)
  if target in {0x146ea09f0,0x146ea1920,0x146ea0ba0}:
   n={0x146ea1920:2,0x146ea0ba0:4}.get(target,d)
   if n>1024 or self.pos+n>len(self.payload):raise ValueError(f'overrun {addr:x}: {self.pos}+{n}>{len(self.payload)}')
   data=self.payload[self.pos:self.pos+n];u.mem_write(c,data)
   self.reads.append(dict(site=hex(addr),offset=self.pos,size=n,hex=data.hex()))
   self.pos+=n;ret=n
  else:
   ret=0
   if target==0x146e8ba20:ret=0x210000
   elif target==0x146d35770:ret=2 # PVF gate group in Elvengard38/2
   elif target in {0x146e920a0,0x146e922e0}:
    try:u.mem_read(d,4)
    except unicorn.UcError:self.missing(u,0,d,4,0,None)
    u.mem_write(d,bytes(4))
  u.reg_write(UC_X86_REG_RAX,ret);u.reg_write(UC_X86_REG_RIP,addr+size)
 def parse(self):
  try:self.call(0x145303260)
  except RuntimeError:
   if not self.done:raise
  if not self.done or self.pos!=len(self.payload):raise ValueError(f'incomplete {self.pos}/{len(self.payload)}')
  return dict(payload_hex=self.payload.hex(),consumed=self.pos,reads=self.reads,
   scope='original NOTI27 non-relay solo parser; all game and UI calls stubbed')

if __name__=='__main__':
 data=bytes(36)
 result=GateOracle(data).parse()
 try:GateOracle(data[:-1]).parse()
 except ValueError as e:result['truncation_failure']=str(e)
 else:raise AssertionError('short packet accepted')
 dest=pathlib.Path(__file__).parent.parent/'dfo-lan/internal/game/protocol/testdata/native_dungeon_gate_cursor.json'
 dest.write_text(json.dumps(result,indent=2),encoding='utf-8')
 print(json.dumps({k:v for k,v in result.items() if k not in ('reads','payload_hex')}))

"""Run the original addition parser's packet-reading control flow in Unicorn.

Only packet readers and nested packet parsers execute. Other game/business
calls are stubbed against private dummy actors. This proves cursor consumption,
not that a live actor is fully initialized or the game UI accepts the packet.
No game process, OS calls, input, network, or original file writes are involved.
"""
from native_cipher_oracle import Oracle
from unicorn.x86_const import *
import capstone, unicorn, struct, json, pathlib

class CursorOracle(Oracle):
 def __init__(self, payload):
  super().__init__()
  self.payload=payload; self.pos=5; self.reads=[]; self.calls={}
  self.md=capstone.Cs(capstone.CS_ARCH_X86,capstone.CS_MODE_64)
  self.u.mem_map(0,0x10000) # inert dummy vtable / empty string, never executed
  self.u.mem_write(0x210000,struct.pack('<Q',0x220000))
  self.u.mem_write(0x230000,struct.pack('<Q',0x231000))
  self.u.hook_add(unicorn.UC_HOOK_CODE,self.step)
 def step(self,u,addr,size,_):
  if addr==0x14563dd09:
   self.done=True; u.emu_stop(); return
  ins=next(self.md.disasm(bytes(u.mem_read(addr,size)),addr))
  if ins.mnemonic!='call':return
  target=int(ins.op_str,16) if ins.op_str.startswith('0x') else 0
  if target in {0x145639c20,0x146d77f90,0x146d77f50,0x1452c1540,0x14563c1f0,0x1456395a0}:return
  c=u.reg_read(UC_X86_REG_RCX); d=u.reg_read(UC_X86_REG_RDX)
  if target in {0x146ea09f0,0x146ea0be0,0x146ea1920,0x146ea0ba0}:
   n={0x146ea1920:2,0x146ea0ba0:4}.get(target,d)
   if n>4096 or self.pos+n>len(self.payload):raise ValueError(f'packet overrun at {addr:x}: {self.pos}+{n}>{len(self.payload)}')
   data=self.payload[self.pos:self.pos+n]
   for page in range(c&~4095,(c+n+4095)&~4095,4096):
    try:u.mem_read(page,1)
    except unicorn.UcError:
     if not self.missing(u,0,page,1,0,None):raise
   u.mem_write(c,data);self.reads.append(dict(site=hex(addr),offset=self.pos,size=n,hex=data.hex()))
   self.pos+=n;ret=n
  else:
   self.calls[hex(target)]=self.calls.get(hex(target),0)+1
   ret=0
   if target in {0x145f0bfa0,0x145637370,0x146e8ba20}:ret=0x210000
   elif target==0x140176170:ret=0x230000
   elif target in {0x146e920a0,0x146e922e0}:u.mem_write(d,b'\0'*4)
   elif addr==0x14563d6a2:ret=0x240000
   # Constructors used only to initialize optional UI helpers.
   elif target in {0x146e93360,0x143c7ae90,0x14363d390,0x145633bf0,0x142f1def0}:ret=c
  u.reg_write(UC_X86_REG_RAX,ret)
  u.reg_write(UC_X86_REG_RIP,addr+size)
 def parse(self):
  self.done=False
  try:self.call(0x14563d400)
  except RuntimeError:
   if not self.done:raise
  if not self.done or self.pos!=len(self.payload):raise ValueError(f'cursor incomplete {self.pos}/{len(self.payload)}')
  return dict(payload_hex=self.payload.hex(),consumed=self.pos,reads=self.reads,
    scope='original native packet-reading branches; game and UI calls stubbed')

if __name__=='__main__':
 p=pathlib.Path(__file__).parent
 # Independent native-parser input: nonzero XP/stat/skill sentinels catch shifts.
 stats=struct.pack('<II',5200,4800)+bytes(78)+bytes([100])+bytes(4)
 data=bytes([1,1,0,0,0])+bytes(250)+struct.pack('<HQI',3,0x1122334455667788,91)+stats
 data+=bytes(1+14+3+8)+bytes([255])
 data+=bytes([1])+struct.pack('<HB',179,7)+bytes(53)
 data+=bytes([2])+struct.pack('<HBHB',174,1,169,1)+bytes(53)
 data+=bytes(4+8+2)
 result=CursorOracle(data).parse()
 # A legacy packet must fail this independent native read contract.
 legacy=data[:361]+data[375:]
 try:CursorOracle(legacy).parse()
 except ValueError as e:result['legacy_failure']=str(e)
 else:raise AssertionError('legacy missing equipment block was accepted')
 dest=p.parent/'dfo-lan/internal/game/protocol/testdata/native_addition_cursor.json'
 dest.write_text(json.dumps(result,indent=2),encoding='utf-8')
 print(json.dumps({k:v for k,v in result.items() if k not in ('reads','payload_hex')},indent=2))

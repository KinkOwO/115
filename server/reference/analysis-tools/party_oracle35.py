"""Current PARTY_INFO reader discovery; no packets sent or live writes."""
from dungeon_map_oracle import PacketOracle
from unicorn.x86_const import *
import struct,json,pathlib

class PartyOracle(PacketOracle):
 def __init__(self,payload,discover=False):
  super().__init__(payload,0x1452f2620,0x1452f8e3c);self.discover=discover;self.effects=[]
  for addr in (0x1452f2826+8+0x93913f2,0x1452f2834+7+0x93913cd):
   try:self.u.mem_read(addr,8)
   except Exception:self.missing(self.u,0,addr,8,0,None)
   self.u.mem_write(addr,struct.pack('<Q',0x250000))
 def step(self,u,addr,size,_):
  ins=next(self.md.disasm(bytes(u.mem_read(addr,size)),addr))
  if ins.mnemonic=='call':
   target=int(ins.op_str,16) if ins.op_str.startswith('0x') else 0
   if target==0x146e920a0 and u.reg_read(UC_X86_REG_RCX)==0x14ef2ca00:
    u.mem_write(u.reg_read(UC_X86_REG_RDX),struct.pack('<I',3));u.reg_write(UC_X86_REG_RIP,addr+size);return
   if 0x145f00000<=target<0x145f20000:
    self.effects.append({'call':hex(target),'site':hex(addr),'d':u.reg_read(UC_X86_REG_RDX),'r8':u.reg_read(UC_X86_REG_R8)})
   if target in (0x146ea09f0,0x146ea0ba0,0x146ea1920) and self.discover:
    n={0x146ea1920:2,0x146ea0ba0:4}.get(target,u.reg_read(UC_X86_REG_RDX))
    if n>256 or self.pos+n>2048:raise ValueError('party discovery bound')
    if self.pos+n>len(self.payload):self.payload+=bytes(self.pos+n-len(self.payload))
   if target in (0x145f0ba60,0x145f0bfa0,0x145f125b0,0x14014f460):
    u.reg_write(UC_X86_REG_RAX,0x260000 if target==0x145f125b0 else 0x250000);u.reg_write(UC_X86_REG_RIP,addr+size);return
  super().step(u,addr,size,_)

if __name__=='__main__':
 p=struct.pack('<HHH6B',1,0,1,0,0,0,0,0,0)
 o=PartyOracle(p,True)
 try:r=o.parse()
 except Exception:
  print('party failed',o.pos,hex(o.u.reg_read(UC_X86_REG_RIP)),o.reads[-8:]);raise
 dest=pathlib.Path(__file__).parent.parent/'dfo-lan/runtime/party-discovery35.json'
 dest.write_text(json.dumps(r,indent=2));print('PARTY_DISCOVERED',r['consumed'],r['reads'])

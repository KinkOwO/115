"""Nonzero gold row: original reader, currency delta and overhead-number calls."""
from item_pickup_oracle import PickupTailOracle
from unicorn.x86_const import *
import json, pathlib, struct

class GoldOracle(PickupTailOracle):
 def __init__(self, payload):
  super().__init__(payload, True)
  self.effects=[]
 def step(self,u,addr,size,_):
  ins=next(self.md.disasm(bytes(u.mem_read(addr,size)),addr))
  if ins.mnemonic=='call':
   target=int(ins.op_str,16) if ins.op_str.startswith('0x') else 0
   if target in (0x145f0ba60,0x145f13700,0x145ad9a90,0x145f12d90):
    ret=1 if target==0x145f12d90 else 0x250000
    u.reg_write(UC_X86_REG_RAX,ret);u.reg_write(UC_X86_REG_RIP,addr+size);return
   if addr==0x1452d2912 or target==0x145a2fbe0:
    self.effects.append({'site':hex(addr),'kind':'gold_delta' if addr==0x1452d2912 else 'overhead_gold','amount':u.reg_read(UC_X86_REG_RDX)&0xffffffff})
  super().step(u,addr,size,_)

if __name__=='__main__':
 payload=bytes([1])+struct.pack('<I',31)+bytes([0])+bytes(35)
 o=GoldOracle(payload);r=o.parse()
 assert r['consumed']==41 and [e['amount'] for e in o.effects]==[31,31],o.effects
 r['effects']=o.effects
 r['scope']='original gold pickup reader and delta/overhead dispatch arguments; local solo party0 and actor getters stubbed; rendering not executed'
 dest=pathlib.Path(__file__).parent.parent/'dfo-lan/internal/game/protocol/testdata/native_gold_pickup30.json'
 dest.write_text(json.dumps(r,indent=2))
 print('NATIVE_GOLD_DELTA_OVERHEAD_PASS',o.effects)

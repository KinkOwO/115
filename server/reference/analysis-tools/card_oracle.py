"""Current client card packet cursors and UI setter arguments, private emulation."""
from settlement_oracle import SettlementOracle
from unicorn.x86_const import *
import struct,json,pathlib

class CardOracle(SettlementOracle):
 def call(self,va,*args):return super().call(va,0,1,0)
 def step(self,u,addr,size,_):
  ins=next(self.md.disasm(bytes(u.mem_read(addr,size)),addr))
  if ins.mnemonic=='call':
   t=int(ins.op_str,16) if ins.op_str.startswith('0x') else 0
   if t in {0x146abbc90,0x146abbc70,0x146ab7b60,0x146ab7990,0x146abbc40,0x146abaf50,0x146aa0ae0}:
    e=dict(setter=hex(t),d=u.reg_read(UC_X86_REG_RDX)&0xffffffff,r8=u.reg_read(UC_X86_REG_R8)&0xffffffff,r9=u.reg_read(UC_X86_REG_R9)&0xffffffff)
    if t==0x146aa0ae0:e['group']=struct.unpack('<I',u.mem_read(u.reg_read(UC_X86_REG_RSP)+0x20,4))[0]
    self.effects.append(e)
  super().step(u,addr,size,_)

if __name__=='__main__':
 dest=pathlib.Path(__file__).parent.parent/'dfo-lan/internal/game/protocol/testdata'
 rows=[('card_scroll',b'',0x145258890,0x1452589ad),('card_layout',struct.pack('<8H',1,*([65535]*7)),0x14524f9e0,0x14524fb2a)]
 for chosen in range(4):
  p=b''.join(bytes([0 if i==chosen else 255,255,0,0]) for i in range(8))
  rows.append((f'card_free_{chosen}',p,0x145246a50,0x145246bd7))
 for state in (1,2):
  for option in range(3):rows.append((f'card_exit_{state}_{option}',bytes([state,option]),0x145244570,0x145244b01))
 base=bytearray(281);struct.pack_into('<II',base,0,500,50);base[167]=1
 struct.pack_into('<I',base,169,1234)
 for off in (265,277):struct.pack_into('<I',base,off,0xffffffff)
 # Current NOTI35 rows include21B item metadata after each ID/amount pair.
 cards=bytes([2])+struct.pack('<II',0,5)+bytes(21)+struct.pack('<II',1012,1)+bytes(21)+bytes(7)
 p=bytes(base[:131])+cards+bytes(base[139:])
 rows.append(('clear_reward_cards',p,0x1452a6c20,0x1452a831f))
 for name,p,start,end in rows:
  o=CardOracle(p,start,end)
  try:r=o.parse()
  except Exception:
   print(name,'pos',o.pos,'rip',hex(o.u.reg_read(UC_X86_REG_RIP)),o.effects[-4:]);raise
  r['effects']=o.effects
  r['scope']='current native packet cursor and UI setter arguments; game/UI bodies stubbed, not live acceptance'
  (dest/f'native_{name}.json').write_text(json.dumps(r,indent=2),encoding='utf-8')
  print(name,r['consumed'],o.effects[-4:])

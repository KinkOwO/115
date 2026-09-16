"""Original NOTI3 reader/writer and room-move guard, private emulation only."""
from dungeon_map_oracle import PacketOracle
from unicorn.x86_const import *
import struct,json,pathlib

class StateOracle(PacketOracle):
 def __init__(self,actor,state):
  super().__init__(struct.pack('<BHB',1,actor,state),0x1453129d0,0x145312c83)
  self.actor=actor
 def step(self,u,addr,size,_):
  ins=next(self.md.disasm(bytes(u.mem_read(addr,size)),addr))
  if ins.mnemonic=='call' and ins.op_str in ('0x145f0bfa0','0x140176170'):
   if ins.op_str=='0x145f0bfa0':assert u.reg_read(UC_X86_REG_RDX)==self.actor
   u.reg_write(UC_X86_REG_RAX,0x250000)
   u.reg_write(UC_X86_REG_RIP,addr+size);return
  super().step(u,addr,size,_)

rows=[]
for state in (0,1):
 o=StateOracle(503,state);r=o.parse()
 value=struct.unpack('<I',o.u.mem_read(0x25002c,4))[0]
 assert value==state
 # Original conditional guard uses precisely the object field written above.
 o.u.reg_write(UC_X86_REG_RAX,0x250000)
 o.u.emu_start(0x144d34be9,0,count=2)
 next_ip=o.u.reg_read(UC_X86_REG_RIP)
 assert next_ip==(0x144d35912 if state==0 else 0x144d34bf3)
 r.update(state=state,stored_state=value,room_guard_next=hex(next_ip));rows.append(r)
p=pathlib.Path(__file__).parent.parent/'dfo-lan/internal/game/protocol/testdata/native_user_state.json'
p.write_text(json.dumps(rows,indent=2),encoding='utf-8')
print('Native NOTI3 actor/state and room guard verified for town0 and dungeon1')

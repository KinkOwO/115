"""Current CMD43 ordinary pickup writer, game getters mocked, no sends."""
from result_request_oracle import ResultWriter
from dungeon_map_oracle import PacketOracle
from unicorn.x86_const import *
import json,pathlib

class PickupWriter(ResultWriter):
 def __init__(self):
  PacketOracle.__init__(self,b'',0x145af4a70,0x145af50e5);self.body=bytearray();self.writes=[]
 def step(self,u,addr,size,_):
  if addr==self.end:self.done=True;u.emu_stop();return
  ins=next(self.md.disasm(bytes(u.mem_read(addr,size)),addr))
  if ins.mnemonic!='call':return
  target=int(ins.op_str,16) if ins.op_str.startswith('0x') else 0
  d=u.reg_read(UC_X86_REG_RDX);ret=0
  if target==0x146d74000:ret=0x260000
  elif target==0x145efafb0:ret=0x250000
  elif target==0x145b8c670:ret=0x11223344
  elif target==0x145b8c7d0:ret=500
  elif target==0x145b8c890:ret=250
  elif target==0x146e9ba90:ret=210
  elif target in {0x146d75cc0,0x146d76180,0x146d75ce0}:
   n={0x146d75cc0:1,0x146d76180:2,0x146d75ce0:4}[target];value=d&((1<<(8*n))-1)
   self.writes.append(dict(site=hex(addr),offset=len(self.body),size=n,value=value));self.body.extend(value.to_bytes(n,'little'))
  u.reg_write(UC_X86_REG_RAX,ret);u.reg_write(UC_X86_REG_RIP,addr+size)

if __name__=='__main__':
 o=PickupWriter()
 try:o.call(o.handler,0x240000,0x270000)
 except RuntimeError:
  if not o.done:raise
 assert o.done
 data=dict(payload_hex=o.body.hex(),length=len(o.body),writes=o.writes,scope='current CMD43 ordinary native writer, scene/getters mocked, no network send')
 dest=pathlib.Path(__file__).parent.parent/'dfo-lan/internal/game/protocol/testdata/native_pickup_request.json'
 dest.write_text(json.dumps(data,indent=2),encoding='utf-8')
 print('pickup request',data)

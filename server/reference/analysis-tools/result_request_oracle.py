"""Current solo CMD46 writer, native writes and control flow, private only."""
from dungeon_map_oracle import PacketOracle
from unicorn.x86_const import *
import struct,json,pathlib

class ResultWriter(PacketOracle):
 def __init__(self):
  super().__init__(b'',0x145e28a2b,0x145e292c8);self.writes=[];self.body=bytearray()
  self.u.reg_write(UC_X86_REG_RBP,0x280000)
  self.u.reg_write(UC_X86_REG_R13,0x270000)
 def step(self,u,addr,size,_):
  if addr==self.end:self.done=True;u.emu_stop();return
  ins=next(self.md.disasm(bytes(u.mem_read(addr,size)),addr))
  if ins.mnemonic!='call':return
  target=int(ins.op_str,16) if ins.op_str.startswith('0x') else 0
  ret=0;d=u.reg_read(UC_X86_REG_RDX)
  if target==0x146d74000:ret=0x260000
  elif target==0x145efafb0:ret=0x250000
  elif target==0x145f13be0:ret=1
  elif target==0x145f13700:ret=0x250000 if d==0 else 0
  elif target in {0x145effac0,0x1459a9080,0x140157c80}:ret=0x250000
  elif target==0x145ce2860:ret=3
  elif ins.op_str=='qword ptr [rdx + 0x21d0]':ret=0xffffffff
  elif target==0x146e920a0:u.mem_write(d,struct.pack('<f',65.0))
  elif target in {0x146d75cc0,0x146d76180,0x146d75ce0}:
   n={0x146d75cc0:1,0x146d76180:2,0x146d75ce0:4}[target]
   value=d&((1<<(n*8))-1);self.writes.append(dict(site=hex(addr),offset=len(self.body),size=n,value=value));self.body.extend(value.to_bytes(n,'little'))
  u.reg_write(UC_X86_REG_RAX,ret);u.reg_write(UC_X86_REG_RIP,addr+size)

if __name__=='__main__':
 o=ResultWriter()
 try:o.call(o.handler)
 except RuntimeError:
  if not o.done:raise
 assert o.done
 data=dict(payload_hex=o.body.hex(),length=len(o.body),writes=o.writes,scope='original solo CMD46 writer; one actor3, stat getters mocked, no packet sent')
 dest=pathlib.Path(__file__).parent.parent/'dfo-lan/internal/game/protocol/testdata/native_play_result_request.json'
 dest.write_text(json.dumps(data,indent=2),encoding='utf-8')
 print('result request length',len(o.body),'writes',o.writes)

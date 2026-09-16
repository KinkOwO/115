from dungeon_map_oracle import PacketOracle
from unicorn.x86_const import *
import struct,json,pathlib
class DirectoryOracle(PacketOracle):
 def __init__(self,p):
  super().__init__(p,0x1451f8efc,0x1451f9356)
  self.u.reg_write(UC_X86_REG_RBP,0x270000);self.u.reg_write(UC_X86_REG_RBX,0);self.rows=[]
 def step(self,u,a,n,_):
  ins=next(self.md.disasm(bytes(u.mem_read(a,n)),a))
  if ins.mnemonic=='call' and ins.op_str.startswith('0x'):
   target=int(ins.op_str,16)
   if target==0x1451fb240:
    size=u.reg_read(UC_X86_REG_R8);dest=u.reg_read(UC_X86_REG_RDX)
    if self.pos+size>len(self.payload):raise ValueError('directory overrun')
    u.mem_write(dest,self.payload[self.pos:self.pos+size]);self.reads.append({'offset':self.pos,'size':size});self.pos+=size
    u.reg_write(UC_X86_REG_RAX,1);u.reg_write(UC_X86_REG_RIP,a+n);return
   if target==0x144d98940:
    self.rows.append(bytes(u.mem_read(u.reg_read(UC_X86_REG_RDX),56)).hex())
  super().step(u,a,n,_)
if __name__=='__main__':
 data=struct.pack('<I',1)+b'first'.ljust(20,b'\0')+struct.pack('<I',1)+b'#LAN Local'.ljust(20,b'\0')+struct.pack('<II',30,1)+b'127.0.0.1'.ljust(16,b'\0')+struct.pack('<I',12345)
 o=DirectoryOracle(data);r=o.parse();r['rows']=o.rows
 assert r['consumed']==76 and len(o.rows)==1
 r['scope']='current channel directory native reader and record setter; string, address and UI operations stubbed'
 p=pathlib.Path(__file__).parent.parent/'dfo-lan/internal/game/protocol/testdata/native_channel_directory.json';p.write_text(json.dumps(r,indent=2))
 print('NATIVE_CHANNEL_DIRECTORY_PASS',r['consumed'],r['reads'])

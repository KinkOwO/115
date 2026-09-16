"""Run the original protobuf field parsers in private Unicorn memory.

Nested message dispatch is stubbed to record bytes; each nested message is
then passed independently through its own original parser. No game/UI calls.
"""
from native_cipher_oracle import Oracle
from unicorn.x86_const import *
import unicorn,struct,json,pathlib

def varint(v):
 b=bytearray()
 while v>127:b.append((v&127)|128);v>>=7
 return bytes(b)+bytes([v])
def field(n,v):return bytes([n*8])+varint(v)
def msg(n,b):return bytes([n*8+2])+varint(len(b))+b

class PBOracle(Oracle):
 def __init__(self,data):
  super().__init__();self.data=data;self.nested=[]
  self.u.mem_write(0x230000,data+bytes(32))
  self.u.mem_write(0x240000,struct.pack('<QQQII',0x230000+len(data),0x230000,1,0,len(data)))
  self.u.mem_write(0x220028,struct.pack('<Q',0x250000))
  self.u.mem_write(0x250000,struct.pack('<Q',256)+struct.pack('<256Q',*([0x260000]*256)))
  self.u.hook_add(unicorn.UC_HOOK_CODE,self.step)
 def step(self,u,a,size,_):
  if a!=0x148586140:return
  p=u.reg_read(UC_X86_REG_R8);v=0;shift=0
  while True:
   b=u.mem_read(p,1)[0];p+=1;v|=(b&127)<<shift;shift+=7
   if b<128:break
  if p+v>0x230000+len(self.data):raise ValueError('nested overrun')
  self.nested.append(bytes(u.mem_read(p,v)).hex())
  sp=u.reg_read(UC_X86_REG_RSP)
  u.reg_write(UC_X86_REG_RAX,p+v);u.reg_write(UC_X86_REG_RIP,struct.unpack('<Q',u.mem_read(sp,8))[0]);u.reg_write(UC_X86_REG_RSP,sp+8)
 def parse(self,fn,offsets):
  self.call(fn,0x220000,0x230000,0x240000)
  assert self.u.reg_read(UC_X86_REG_RAX)==0x230000+len(self.data)
  return {'payload_hex':self.data.hex(),'fields':[struct.unpack('<I',self.u.mem_read(0x220000+x,4))[0] for x in offsets],'nested':self.nested}

if __name__=='__main__':
 skills=[(179,7,65535),(174,1,65535),(169,1,0),(46,1,1),(190,1,65535),(5,1,2),(511,1,65535),(452,1,65535)]
 items=[field(1,slot)+field(2,id)+field(3,level) for id,level,slot in skills]
 tree=field(1,0)+field(2,0)+field(4,0)+b''.join(msg(3,b) for b in items)
 body=field(1,1)+msg(2,tree)+msg(2,tree)
 result={'scope':'original protobuf field parsers; nested dispatch isolated; UI not executed','payload_hex':(struct.pack('<I',len(body))+body).hex()}
 result['top']=PBOracle(body).parse(0x1401a3300,[0x30,0x20]);assert result['top']['fields']==[1,2]
 o=PBOracle(tree);result['tree']=o.parse(0x1401a3810,[0x60,0x64,0x68,0x20]);assert result['tree']['fields']==[0,0,0,8]
 # Original IsInitialized additionally requires tree fields1,2,4 and
 # every nested skill field1,2,3. Omitted scalar zero is not equivalent.
 o.u.mem_write(0x260010,struct.pack('<I',7))
 assert o.call(0x1401a1320,0x220000)&255==1
 o.u.mem_write(0x220010,struct.pack('<I',3))
 assert o.call(0x1401a1320,0x220000)&255==0
 result['required_field4_verified']=True
 result['skills']=[PBOracle(b).parse(0x1401a3500,[0x28,0x2c,0x30]) for b in items]
 assert [r['fields'] for r in result['skills']]==[[slot,id,level] for id,level,slot in skills]
 dest=pathlib.Path(__file__).parent.parent/'dfo-lan/internal/game/protocol/testdata/native_self_skills.json'
 dest.write_text(json.dumps(result,indent=2),encoding='utf-8');print('native protobuf skill fields verified: 2 trees, 8 skills each')

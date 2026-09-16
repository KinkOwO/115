"""Bounded native NOTI28/29 cursor validation, with game/UI calls stubbed."""
from dungeon_gate_oracle import GateOracle
from unicorn.x86_const import *
import capstone,unicorn,struct,json,pathlib
from native_cipher_oracle import Oracle

def verify_room_predicates(coordinates):
 # Execute the actual coordinate comparisons after decoding NOTI28. Only
 # the protected integer accessor is replaced by already-observed values.
 o=Oracle();u=o.u;obj=0x250000
 def read_value(u,addr,size,_):
  if addr!=0x146e920a0:return
  source=u.reg_read(UC_X86_REG_RCX)-obj
  if hex(source) not in coordinates:raise AssertionError(hex(source))
  u.mem_write(u.reg_read(UC_X86_REG_RDX),struct.pack('<I',coordinates[hex(source)]))
  sp=u.reg_read(UC_X86_REG_RSP)
  ret=struct.unpack('<Q',u.mem_read(sp,8))[0]
  u.reg_write(UC_X86_REG_RSP,sp+8);u.reg_write(UC_X86_REG_RIP,ret)
 u.hook_add(unicorn.UC_HOOK_CODE,read_value)
 rows=[]
 for xy in [(0,1),(1,1),(1,0),(2,0),(3,0)]:
  boss=bool(o.call(0x145b34090,obj,*xy)&255)
  hell=bool(o.call(0x145b35340,obj,*xy)&255)
  assert boss==(xy==(3,0)) and not hell,(xy,boss,hell)
  rows.append(dict(xy=xy,boss=boss,random_hell=hell))
 return rows

class PacketOracle(GateOracle):
 def __init__(self,payload,handler,end):
  super().__init__(payload);self.handler=handler;self.end=end;self.spawns=[];self.coordinates={}
 def step(self,u,addr,size,_):
  if addr==self.end:self.done=True;u.emu_stop();return
  ins=next(self.md.disasm(bytes(u.mem_read(addr,size)),addr))
  if ins.mnemonic!='call':return
  target=int(ins.op_str,16) if ins.op_str.startswith('0x') else 0
  c=u.reg_read(UC_X86_REG_RCX);d=u.reg_read(UC_X86_REG_RDX)
  if target in {0x146ea09f0,0x146ea0be0,0x146ea1920,0x146ea0ba0}:
   n={0x146ea1920:2,0x146ea0ba0:4}.get(target,d)
   if n>1024 or self.pos+n>len(self.payload):raise ValueError(f'overrun {addr:x}: {self.pos}+{n}>{len(self.payload)}')
   try:u.mem_read(c,max(n,1))
   except unicorn.UcError:self.missing(u,0,c,max(n,1),0,None)
   data=self.payload[self.pos:self.pos+n];u.mem_write(c,data)
   self.reads.append(dict(site=hex(addr),offset=self.pos,size=n,hex=data.hex()));self.pos+=n;ret=n
  else:
   ret=0
   if target==0x145b0d910:
    stack=u.reg_read(UC_X86_REG_RSP)
    self.spawns.append(dict(source_index=d,entity=u.reg_read(UC_X86_REG_R8),template=u.reg_read(UC_X86_REG_R9),level=struct.unpack('<I',u.mem_read(stack+0x20,4))[0],rank=struct.unpack('<I',u.mem_read(stack+0x28,4))[0],team=struct.unpack('<I',u.mem_read(stack+0x50,4))[0]))
   if target in {0x146e8ba20,0x14520cfb0,0x140157c80,0x145b2f650}:ret=0x250000
   elif target==0x1452baa60:
    u.mem_write(c,struct.pack('<QQQ',0x270000,0x270000,0x278000))
   elif target in {0x146e920a0,0x146e922e0}:
    try:u.mem_read(d,4)
    except unicorn.UcError:self.missing(u,0,d,4,0,None)
    value=bytes(u.mem_read(c,4)) if target==0x146e922e0 else bytes(4)
    if target==0x146e922e0 and d-0x250000 in (0x1934,0x193c,0x1944,0x194c):
     self.coordinates[hex(d-0x250000)]=struct.unpack('<I',value)[0]
    u.mem_write(d,value)
  u.reg_write(UC_X86_REG_RAX,ret);u.reg_write(UC_X86_REG_RIP,addr+size)
 def parse(self):
  try:self.call(self.handler)
  except RuntimeError:
   if not self.done:raise
  if not self.done or self.pos!=len(self.payload):raise ValueError(f'incomplete {self.pos}/{len(self.payload)} at {self.u.reg_read(UC_X86_REG_RIP):x}')
  return dict(payload_hex=self.payload.hex(),consumed=self.pos,reads=self.reads,spawns=self.spawns,coordinates=self.coordinates,scope='original native packet reader, coordinate setter and spawn arguments; game, map and UI calls stubbed')

if __name__=='__main__':
 p=pathlib.Path(__file__).parent.parent/'dfo-lan/internal/game/protocol/testdata'
 info=struct.pack('<IBHBBBBBB',3,0,0,1,3,0,255,255,0)+bytes(1+4+1)+struct.pack('<I',0xffffffff)+bytes(3+3+1+1+6+4)
 entry=bytes([0,1,0])+struct.pack('<I',12345)+bytes(2)+struct.pack('<I',0xffffffff)+bytes(4)+bytes([255])*8+bytes(6)+bytes([1])+struct.pack('<I',76121)
 # One monster record with distinct index, identity and level sentinels.
 entry+=bytes([1])+struct.pack('<HIHI5BIB',0,0,1,109014870,3,3,0,0,255,100,0)+bytes(3)+bytes([255])
 for name,data,start,end in [('dungeon_info',info,0x1452ac840,0x1452adf4e),('start_map',entry,0x1452b7100,0x1452b8c7c)]:
  result=PacketOracle(data,start,end).parse()
  if name=='dungeon_info':
   assert result['coordinates']=={'0x1934':3,'0x193c':0,'0x1944':255,'0x194c':255}
   result['native_room_predicates']=verify_room_predicates(result['coordinates'])
  if name=='start_map':assert result['spawns']==[dict(source_index=0,entity=1,template=109014870,level=3,rank=3,team=100)]
  try:PacketOracle(data[:-1],start,end).parse()
  except ValueError as e:result['truncation_failure']=str(e)
  else:raise AssertionError('short packet accepted')
  (p/f'native_{name}_cursor.json').write_text(json.dumps(result,indent=2),encoding='utf-8')
  print(name,json.dumps({k:v for k,v in result.items() if k not in ('reads','payload_hex')}))
 friendly=entry[:36]+bytes([2])+struct.pack('<HIHI5BIB',0,0,1,109014870,3,3,0,0,255,100,0)+struct.pack('<HIHI5BIB',0,1,2,109014870,3,0,0,0,255,0,0)+bytes(3)+bytes([255])
 result=PacketOracle(friendly,0x1452b7100,0x1452b8c7c).parse()
 assert [m['team'] for m in result['spawns']]==[100,0]
 (p/'native_start_map_teams.json').write_text(json.dumps(result,indent=2),encoding='utf-8')
 print('mixed teams',result['spawns'])

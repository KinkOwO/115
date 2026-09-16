"""Original channel number extraction; CRT decimal conversion is substituted."""
from native_cipher_oracle import Oracle
from unicorn.x86_const import *
import unicorn,struct,json,pathlib

def number(name):
 o=Oracle();u=o.u;obj=0x250000
 raw=name.encode('utf-16le')+bytes(2)
 if len(name)<8:u.mem_write(obj,raw.ljust(16,b'\0'));cap=7
 else:u.mem_write(0x260000,raw);u.mem_write(obj,struct.pack('<Q',0x260000)+bytes(8));cap=len(name)
 u.mem_write(obj+16,struct.pack('<QQ',len(name),cap))
 digits=[]
 def hook(u,a,n,_):
  if a not in (0x148ab91bc,0x148860610):return
  if a==0x148ab91bc:
   ptr=u.reg_read(UC_X86_REG_RCX);text=''
   for i in range(7):
    c=int.from_bytes(u.mem_read(ptr+i*2,2),'little')
    if not c:break
    text+=chr(c)
   digits.append(text);u.reg_write(UC_X86_REG_RAX,int(text or '0'))
  sp=u.reg_read(UC_X86_REG_RSP);ret=struct.unpack('<Q',u.mem_read(sp,8))[0]
  u.reg_write(UC_X86_REG_RSP,sp+8);u.reg_write(UC_X86_REG_RIP,ret)
 u.hook_add(unicorn.UC_HOOK_CODE,hook)
 value=o.call(0x1451fa5e0,obj)
 return {'name':name,'digits':digits,'channel_id':value}

if __name__=='__main__':
 rows=[number(x) for x in ('#LAN Local','#1','#23')]
 assert [r['channel_id'] for r in rows]==[0,1,23],rows
 dest=pathlib.Path(__file__).parent.parent/'dfo-lan/internal/game/protocol/testdata/native_channel_numbers31.json'
 dest.write_text(json.dumps(rows,indent=2));print('NATIVE_CHANNEL_NUMBER_PASS',rows)

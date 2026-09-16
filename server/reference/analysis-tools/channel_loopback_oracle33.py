"""Execute the current auto-select IP exclusion branch, no OS/game execution."""
from native_cipher_oracle import Oracle
from unicorn.x86_const import *
import unicorn,struct,json,pathlib

def selected(ip):
 o=Oracle();u=o.u;obj=0x250000;data=0x260000;literal=0x270000;bp=0x280000
 u.mem_write(data,(ip+'\0').encode('utf-16le'))
 u.mem_write(literal,'127.0.0.1\0'.encode('utf-16le'))
 u.mem_write(obj,struct.pack('<QQQQ',data,0,len(ip),len(ip)))
 u.reg_write(UC_X86_REG_RAX,obj);u.reg_write(UC_X86_REG_RDI,literal)
 u.reg_write(UC_X86_REG_RBP,bp);u.reg_write(UC_X86_REG_R12,0)
 u.mem_write(bp+7,struct.pack('<QQQQ',data,0,len(ip),len(ip)))
 outcome=[]
 def hook(u,a,n,_):
  if a in (0x145503b65,0x145503d1d):outcome.append(a==0x145503b65);u.emu_stop()
  if a==0x146e9f3a0:
   sp=u.reg_read(UC_X86_REG_RSP);ret=struct.unpack('<Q',u.mem_read(sp,8))[0]
   u.reg_write(UC_X86_REG_RSP,sp+8);u.reg_write(UC_X86_REG_RIP,ret)
 u.hook_add(unicorn.UC_HOOK_CODE,hook)
 try:o.call(0x145503aa3)
 except RuntimeError:
  if not outcome:raise
 return {'ip':ip,'passes_ip_filter':outcome[0]}

if __name__=='__main__':
 rows=[selected(ip) for ip in ('127.0.0.1','127.0.0.2')]
 assert [r['passes_ip_filter'] for r in rows]==[False,True],rows
 dest=pathlib.Path(__file__).parent.parent/'dfo-lan/internal/game/protocol/testdata/native_channel_ip_filter33.json'
 dest.write_text(json.dumps(rows,indent=2));print('NATIVE_CHANNEL_IP_FILTER_PASS',rows)

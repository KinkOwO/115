"""Offline evaluation of captured monster HP descriptors using original getters."""
from native_cipher_oracle import Oracle
from unicorn.x86_const import *
import pathlib,json,sys,struct,unicorn
p=pathlib.Path(sys.argv[1]); rows=[];seen=set()
for line in p.read_text().splitlines():
 r=json.loads(line)
 if r.get('kind')!='monster_hp_descriptor' or r['team']!=100:continue
 key=(r['entity'],r['raw_hex'])
 if key in seen:continue
 seen.add(key);o=Oracle();raw=bytes.fromhex(r['raw_hex']);o.u.mem_write(0x250000,raw)
 def encoded_set(u,a,n,_):
  if a!=0x146e922e0:return
  value=int.from_bytes(u.mem_read(u.reg_read(UC_X86_REG_RCX),4),'little')
  u.mem_write(u.reg_read(UC_X86_REG_RDX),struct.pack('<I',((value+4)&0xffffffff)^0x1f2a025c))
  sp=u.reg_read(UC_X86_REG_RSP);ret=struct.unpack('<Q',u.mem_read(sp,8))[0]
  u.reg_write(UC_X86_REG_RSP,sp+8);u.reg_write(UC_X86_REG_RIP,ret)
 o.u.hook_add(unicorn.UC_HOOK_CODE,encoded_set)
 try:
  effective=o.call(0x147220e40,0x250000,0)
  unscaled=o.call(0x147221040,0x250000,0)
  secondary=o.call(0x147221db0,0x250000,0)
 except Exception as e:
  print('getter error',r['entity'],e,hex(o.u.reg_read(UC_X86_REG_RIP)));raise
 rows.append({'entity':r['entity'],'time':r['time'],'effective_147220e40':effective,'unscaled_147221040':unscaled,'secondary_147221db0':secondary})
out=p.parent/'monster-hp-evaluated35.json';out.write_text(json.dumps(rows,indent=2))
print(json.dumps(rows[:12]));print('samples',len(rows),'effective_range',[min(r['effective_147220e40'] for r in rows),max(r['effective_147220e40'] for r in rows)])

"""Exact-build boss completion writer/reader in a private emulator."""
from dungeon_map_oracle import PacketOracle
from native_cipher_oracle import Oracle
from unicorn.x86_const import *
import capstone, unicorn, struct, json, pathlib

def request():
 o=Oracle();u=o.u;md=capstone.Cs(capstone.CS_ARCH_X86,capstone.CS_MODE_64)
 body=bytearray();writes=[]
 u.reg_write(UC_X86_REG_R15,0x250000)
 def step(u,addr,size,_):
  if addr==0x145c34fcb:u.emu_stop();return
  ins=next(md.disasm(bytes(u.mem_read(addr,size)),addr))
  if ins.mnemonic!='call':return
  t=int(ins.op_str,16) if ins.op_str.startswith('0x') else 0
  ret=0
  if t==0x146d74000:ret=0x260000
  elif t==0x145b8c670:ret=3 if addr==0x145c34f76 else 0x1234
  elif t==0x145f001d0:ret=0x12345678
  elif t in (0x146d76180,0x146d75ce0):
   n=2 if t==0x146d76180 else 4
   v=u.reg_read(UC_X86_REG_RDX)&((1<<(8*n))-1)
   writes.append(dict(site=hex(addr),offset=len(body),size=n,value=v))
   body.extend(v.to_bytes(n,'little'))
  u.reg_write(UC_X86_REG_RAX,ret);u.reg_write(UC_X86_REG_RIP,addr+size)
 u.hook_add(unicorn.UC_HOOK_CODE,step)
 try:o.call(0x145c34f4d)
 except RuntimeError:
  if u.reg_read(UC_X86_REG_RIP)!=0x145c34fcb:raise
 assert body==struct.pack('<HHII',3,0x1234,0,0x12345678)
 return dict(payload_hex=body.hex(),writes=writes,scope='native CMD117 writer; actor identity and check accessor mocked; no process or network')

if __name__=='__main__':
 dest=pathlib.Path(__file__).parent.parent/'dfo-lan/internal/game/protocol/testdata'
 r=PacketOracle(bytes.fromhex('01013412'),0x1452a5810,0x1452a5b4f).parse()
 try:PacketOracle(bytes.fromhex('010134'),0x1452a5810,0x1452a5b4f).parse()
 except ValueError as e:r['truncation_failure']=str(e)
 else:raise AssertionError('short boss reply accepted')
 for name,data in [('boss_check_request',request()),('boss_check_response',r)]:
  (dest/f'native_{name}.json').write_text(json.dumps(data,indent=2),encoding='utf-8')
  print(name,data['payload_hex'])

"""Bounded x64 emulation of pure cipher routines from the supplied build.

No entry point, OS API, game process, UI, or network is executed. Only mapped
PE pages and a private emulator stack/buffers are accessible. Used to generate
independent golden vectors for the portable Go implementations.
"""
import pathlib,sys,struct,json,hashlib
p=pathlib.Path(__file__).parent;sys.path.insert(0,str(p/'python_lib'))
import pefile,unicorn
from unicorn.x86_const import *
class Oracle:
 def __init__(self):
  self.pe=pefile.PE(r'F:\dnfop\DFO\DFO.exe',fast_load=True)
  self.base=self.pe.OPTIONAL_HEADER.ImageBase
  self.u=unicorn.Uc(unicorn.UC_ARCH_X86,unicorn.UC_MODE_64)
  self.u.mem_map(0x100000,0x1000)
  self.u.mem_map(0x200000,0x100000)
  self.u.hook_add(unicorn.UC_HOOK_MEM_UNMAPPED,self.missing)
 def missing(self,u,access,addr,size,value,user):
  if not self.base<=addr<self.base+self.pe.OPTIONAL_HEADER.SizeOfImage:return False
  start=addr&~4095
  u.mem_map(start,4096)
  u.mem_write(start,self.pe.get_data(start-self.base,4096).ljust(4096,b'\0'))
  return True
 def call(self,va,*args):
  regs=[UC_X86_REG_RCX,UC_X86_REG_RDX,UC_X86_REG_R8,UC_X86_REG_R9]
  for reg,val in zip(regs,args):self.u.reg_write(reg,val)
  self.u.reg_write(UC_X86_REG_RSP,0x2ff008)
  self.u.mem_write(0x2ff008,struct.pack('<Q',0x100000))
  for i,value in enumerate(args[4:]):self.u.mem_write(0x2ff030+i*8,struct.pack('<Q',value))
  self.u.emu_start(va,0x100000,timeout=5000000,count=1000000)
  if self.u.reg_read(UC_X86_REG_RIP)!=0x100000:raise RuntimeError('cipher execution budget exhausted')
  return self.u.reg_read(UC_X86_REG_EAX)
 def cipher(self,key,plain,setup,enc,dec,rounds,blocksize):
  u=self.u;u.mem_write(0x210000,key)
  code=self.call(setup,0x210000,len(key),rounds,0x220000)
  if code:raise RuntimeError(f'key setup returned {code:x}')
  u.mem_write(0x230000,plain)
  for off in range(0,len(plain),blocksize):
   if self.call(enc,0x230000+off,0x240000+off,0x220000):raise RuntimeError('encryption failed')
  encrypted=bytes(u.mem_read(0x240000,len(plain)))
  for off in range(0,len(plain),blocksize):
   if self.call(dec,0x240000+off,0x250000+off,0x220000):raise RuntimeError('decryption failed')
  recovered=bytes(u.mem_read(0x250000,len(plain)))
  if recovered!=plain:raise RuntimeError('native roundtrip failed')
  return encrypted

if __name__=='__main__':
 oracle=Oracle();rows=[]
 algorithms=[('noekeon',16,16,0x146d97090,0x146d96c40,0x146d969e0,16),('skipjack',10,8,0x146d98470,0x146d97a60,0x146d97560,32),('cast5',16,8,0x146d93770,0x146d93750,0x146d93730,16),('xtea_le',16,8,0x146d9a0d0,0x146d99f60,0x146d99bc0,32)]
 for name,keylen,size,setup,enc,dec,rounds in algorithms:
  for seed in [0,1,0x47]:
   key=bytes((i+seed)&255 for i in range(keylen));plain=bytes((i*7+seed)&255 for i in range(size))
   cipher=oracle.cipher(key,plain,setup,enc,dec,rounds,size)
   rows.append(dict(algorithm=name,key=key.hex(),plain=plain.hex(),cipher=cipher.hex(),setup=hex(setup),encrypt=hex(enc),decrypt=hex(dec)))
 for seed in [0,1,0x47]:
  key=bytes((i+seed)&255 for i in range(60));plain=bytes((i*7+seed)&255 for i in range(16))
  oracle.u.mem_write(0x220000,key[:32]);oracle.call(0x146d97410,0x220000,32)
  oracle.u.mem_write(0x230000,plain);oracle.call(0x146d97340,0x220000,0x230000,0x240000)
  cipher=bytes(oracle.u.mem_read(0x240000,16))
  oracle.call(0x146d97250,0x220000,0x240000,0x250000)
  assert bytes(oracle.u.mem_read(0x250000,16))==plain
  rows.append(dict(algorithm='dfo_rc6',key=key.hex(),plain=plain.hex(),cipher=cipher.hex()))
 (p/'native_cipher_vectors.json').write_text(json.dumps(rows,indent=2))
 print(json.dumps(rows,indent=2))

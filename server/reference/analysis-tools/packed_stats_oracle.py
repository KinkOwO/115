"""Validate the exact 91-byte wire struct in native, pure-function emulation."""
from native_cipher_oracle import Oracle
from unicorn.x86_const import UC_X86_REG_GS_BASE
import pathlib, struct, json
formats=['I','I']+['H']*4+['h']*4+['h']*19+['i','h','h','I','H','H','h','h','i','B','f']
rows=[]
for seed in (1,17,91):
 # The numeric protector queries a C++ TLS initialization epoch before its
 # pure transform. Provide a private TLS block with an initialized epoch;
 # no instructions are patched or API hooks substituted. A fresh emulator
 # per vector leaves the incidental sampling counter below its threshold.
 o=Oracle()
 o.u.reg_write(UC_X86_REG_GS_BASE,0x260000)
 o.u.mem_write(0x260058,struct.pack('<Q',0x261000))
 o.u.mem_write(0x261000,struct.pack('<Q',0x200000))
 o.u.mem_write(0x266b0c,struct.pack('<I',0x7fffffff))
 values=[]
 for i,fmt in enumerate(formats):
  if fmt=='f':v=seed/8
  elif fmt=='B':v=100
  elif fmt in ('h','i'):v=-(i*13+seed)
  else:v=i*311+seed
  values.append(v)
 raw=b''.join(struct.pack('<'+fmt,v) for fmt,v in zip(formats,values))
 assert len(raw)==91
 o.u.mem_write(0x210000,raw)
 o.call(0x147555f80,0x220000,0x210000)
 expanded=[]
 for i,(fmt,value) in enumerate(zip(formats,values)):
  o.call(0x146e920a0,0x220000+i*8,0x230000)
  word=struct.unpack('<I',bytes(o.u.mem_read(0x230000,4)))[0]
  expected=(struct.unpack('<I',struct.pack('<f',value))[0] if fmt=='f' else value & 0xffffffff)
  assert word==expected,(i,hex(word),hex(expected))
  expanded.append(word)
 rows.append(dict(wire_hex=raw.hex(),expanded_words=expanded,values=values,environment='private initialized TLS epoch; original native conversion and protector/getter instructions'))
dest=pathlib.Path(__file__).parent.parent/'dfo-lan/internal/game/protocol/testdata/native_packed91.json'
dest.parent.mkdir(parents=True,exist_ok=True)
dest.write_text(json.dumps(rows,indent=2))
print(f'packed91 native expansion/getter verified: {len(rows)} vectors, {len(formats)} fields each')

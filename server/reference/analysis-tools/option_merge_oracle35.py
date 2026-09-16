"""Verify sparse account options against original constructors/merge/getter.

Private emulation only. Source defaults are distinct sentinel values so this
checks that setting option 198 cannot zero or replace other native defaults.
"""
from native_cipher_oracle import Oracle
from unicorn.x86_const import *
from pathlib import Path
import struct, json

def verify(value):
 o=Oracle(); u=o.u
 u.reg_write(UC_X86_REG_GS_BASE,0x200000)
 u.mem_write(0x200058,struct.pack('<Q',0x280000))
 profile=0x210000; defaults=profile+8; account=profile+0x1ce0
 character=profile+0x2b20; merged=profile+0x38f3
 o.call(0x1475757f0,account)
 payload=bytearray(u.mem_read(account,3648))
 assert payload[:6]==bytes.fromhex('0000ffffffff')
 payload[0]=1
 struct.pack_into('<H',payload,2+198*2,value)
 payload[574+198]=1
 u.mem_write(account,bytes(payload))
 o.call(0x147575950,character)
 o.call(0x147575010,defaults)
 u.mem_write(defaults,bytes([1,0]))
 for i in range(286):
  u.mem_write(defaults+2+i*2,struct.pack('<H',1000+i))
 u.mem_write(defaults+574,bytes([1])*286)
 o.call(0x14757abe0,1,account,character,defaults,merged)
 actual=struct.unpack('<286H',u.mem_read(merged+2,572))
 expected=tuple(value if i==198 else 1000+i for i in range(286))
 assert actual==expected
 o.call(0x1403d4720,profile,198)
 pointer=u.reg_read(UC_X86_REG_RAX)
 assert struct.unpack('<H',u.mem_read(pointer,2))[0]==value
 return {'payload_hex':payload.hex(),'guide_option':198,'value':value,
         'other_285_defaults_preserved':True,'native_getter_reads_override':True}

if __name__=='__main__':
 rows=[verify(0),verify(1)]
 dest=Path(__file__).parent.parent/'dfo-lan/internal/game/protocol/testdata/native_account_options35.json'
 dest.write_text(json.dumps({'scope':'native constructor, sparse core merge and getter; no UI or network', 'cases':rows},indent=2))
 print('NATIVE_OPTIONS_PASS sparse_override=198 untouched_defaults=285 values=0,1')

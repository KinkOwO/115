"""Current eligibility predicate against captured owned-client channel rows."""
from native_cipher_oracle import Oracle
from unicorn.x86_const import *
import unicorn,struct,json,pathlib
root=pathlib.Path(__file__).parent.parent/'dfo-lan'
captured=json.loads((root/'runtime/channel-candidates34.json').read_text())

def check(raw):
 o=Oracle();u=o.u;obj=0x250000
 u.mem_write(obj,raw)
 # Keep the already-created profile singleton on the initialized branch.
 glob=0x144d99970+7+0x989a8b9
 o.missing(u,0,glob,8,0,None);u.mem_write(glob,struct.pack('<Q',0x280000))
 def hook(u,a,n,_):
  if a in (0x1403f4fb0,0x144da88a0,0x144da7bc0,0x144da7d00,0x144da7d30):
   # Current live startup logs identify profile19 (DEVZONE).
   # Isolate type eligibility from unrelated global policy managers and
   # capacity. Existing channel rows have30 capacity/1user and group0/1.
   u.reg_write(UC_X86_REG_RAX,19 if a==0x1403f4fb0 else 0)
   sp=u.reg_read(UC_X86_REG_RSP);ret=struct.unpack('<Q',u.mem_read(sp,8))[0]
   u.reg_write(UC_X86_REG_RSP,sp+8);u.reg_write(UC_X86_REG_RIP,ret)
 u.hook_add(unicorn.UC_HOOK_CODE,hook)
 try:return bool(o.call(0x144d998b0,obj)&255)
 except Exception:
  print('failed at',hex(u.reg_read(UC_X86_REG_RIP)), 'stack', bytes(u.mem_read(u.reg_read(UC_X86_REG_RSP),32)).hex());raise

if __name__=='__main__':
 rows=[]
 for c in captured['channels']:
  raw=bytes.fromhex(c['raw_hex']);kind=struct.unpack_from('<I',raw,12)[0]
  rows.append({'kind':kind,'eligible':check(raw),'variant':'captured'})
 raw=bytearray.fromhex(captured['channels'][-1]['raw_hex']);struct.pack_into('<I',raw,12,22)
 rows.append({'kind':22,'eligible':check(bytes(raw)),'variant':'only_type_changed_in_private_snapshot'})
 assert [x['eligible'] for x in rows]==[False,False,False,True],rows
 (root/'internal/game/protocol/testdata/native_channel_eligibility34.json').write_text(json.dumps(rows,indent=2))
 print('NATIVE_CHANNEL_ELIGIBILITY_PASS',rows)

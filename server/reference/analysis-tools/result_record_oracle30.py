"""Verify personal-record announcement receives time and respects no-update flag."""
from settlement_oracle import SettlementOracle
from unicorn.x86_const import *
import pathlib,json,struct

class RecordOracle(SettlementOracle):
 def step(self,u,addr,size,_):
  if addr==0x1452b1b6a:
   u.mem_write(u.reg_read(UC_X86_REG_RDX),struct.pack('<I',3))
   u.reg_write(UC_X86_REG_RIP,addr+size);return
  ins=next(self.md.disasm(bytes(u.mem_read(addr,size)),addr))
  if ins.mnemonic=='call' and ins.op_str in ('0x145e29570','0x145e29890'):
   self.effects.append({'setter':ins.op_str,'value':u.reg_read(UC_X86_REG_RDX)&0xffffffff})
  super().step(u,addr,size,_)

if __name__=='__main__':
 dest=pathlib.Path(__file__).parent.parent/'dfo-lan/internal/game/protocol/testdata'
 for improved in (True,False):
  payload=struct.pack('<BIBBBBH5I',50,60000,0,50,1,1,3,60000,0 if improved else 0xffffffff,0,0,0xffffffff)
  o=RecordOracle(payload,0x1452b1960,0x1452b1d0e);r=o.parse()
  timer=[x['value'] for x in o.effects if x.get('setter')=='0x145e29570']
  assert timer==([60000] if improved else []),o.effects
  r['effects']=o.effects
  r['truncation_failure']=''
  try:RecordOracle(payload[:-1],0x1452b1960,0x1452b1d0e).parse()
  except ValueError as e:r['truncation_failure']=str(e)
  assert r['truncation_failure']
  (dest/('native_play_result_record30.json' if improved else 'native_play_result_no_record30.json')).write_text(json.dumps(r,indent=2))
  print('NATIVE_PERSONAL_RECORD_PASS',improved,timer)

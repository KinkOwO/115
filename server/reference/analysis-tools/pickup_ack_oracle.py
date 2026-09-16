"""Current pickup ACK success and failure, private native cursor proof."""
from dungeon_map_oracle import PacketOracle
from unicorn.x86_const import *
import json,pathlib,struct

if __name__=='__main__':
 dest=pathlib.Path(__file__).parent.parent/'dfo-lan/internal/game/protocol/testdata'
 for success in [True,False]:
  body=b'' if success else struct.pack('<I',0x11223344)
  o=PacketOracle(body,0x145244e00,0x145245860)
  o.u.reg_write(UC_X86_REG_RDX,int(success));o.u.reg_write(UC_X86_REG_R8,4)
  result=o.parse();result['scope']='native CMD43 after dispatcher success/error fields, game/UI stubbed'
  (dest/f'native_pickup_ack_{int(success)}.json').write_text(json.dumps(result,indent=2),encoding='utf-8')
  print('pickup ack',success,result['consumed'])

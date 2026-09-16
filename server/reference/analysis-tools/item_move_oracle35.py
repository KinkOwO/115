"""Current item-space request writes and ACK cursor, private emulation only."""
from dungeon_map_oracle import PacketOracle
from unicorn.x86_const import *
import struct,json,pathlib
dest=pathlib.Path(__file__).parent.parent/'dfo-lan/internal/game/protocol/testdata'
for name,p,a,z in [('item_move_success35',struct.pack('<BHI BHB',0,9,1,3,19,0),0x1452837eb,0x14528384e),('item_move_failure35',struct.pack('<BBIB',0,3,0,0),0x14528559c,0x1452855e5)]:
 o=PacketOracle(p,a,z);o.u.reg_write(UC_X86_REG_RBP,0x280000);o.u.reg_write(UC_X86_REG_RSI,0xffffffffffffffff)
 r=o.parse();r['scope']='current success/refusal packet cursor; stops before inventory mutation/UI methods'
 (dest/('native_'+name+'.json')).write_text(json.dumps(r,indent=2));print(name,r['consumed'])

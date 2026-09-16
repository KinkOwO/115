"""Current native CMD637 empty pending-deletion list reader, private memory only."""
from dungeon_map_oracle import PacketOracle
import json,pathlib
class PendingOracle(PacketOracle):
 def call(self,va,*args):return super().call(va,0,1,0)
if __name__=='__main__':
 r=PendingOracle(bytes([0]),0x145250040,0x14525031d).parse()
 assert r['consumed']==1
 r['scope']='current native empty pending deletion reader; UI bodies stubbed'
 dest=pathlib.Path(__file__).parent.parent/'dfo-lan/internal/game/protocol/testdata/native_delete_pending.json'
 dest.write_text(json.dumps(r,indent=2),encoding='utf-8')
 print('DELETE_PENDING_NATIVE_PASS consumed=1')

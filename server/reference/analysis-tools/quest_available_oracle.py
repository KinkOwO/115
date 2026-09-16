from skill_pb_oracle import PBOracle,field,msg,varint
import struct,json,pathlib
if __name__=='__main__':
 ids=[4873,3146];body=field(2,5)+field(3,0)+msg(4,b''.join(varint(i) for i in ids))
 o=PBOracle(body)
 o.u.mem_write(0x220018,struct.pack('<IIQ',0,32,0x260000))
 r=o.parse(0x1401a1d70,[0x2c,0x30,0x18])
 got=list(struct.unpack('<2I',o.u.mem_read(0x260000,8)))
 assert r['fields']==[5,0,2] and got==ids
 r.update(ids=got,payload_hex=(struct.pack('<I',len(body))+body).hex(),scope='current NOTI21 generated field parser, source IDs and required level; no UI calls')
 dest=pathlib.Path(__file__).parent.parent/'dfo-lan/internal/game/protocol/testdata/native_available_quests.json'
 dest.write_text(json.dumps(r,indent=2),encoding='utf-8')
 print('NATIVE_AVAILABLE_QUESTS_PASS',r['fields'],got)

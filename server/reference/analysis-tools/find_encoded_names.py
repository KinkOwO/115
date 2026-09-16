import pathlib,runpy,contextlib,io,struct,bisect,json
p=pathlib.Path(__file__).parent
with contextlib.redirect_stdout(io.StringIO()):n=runpy.run_path(str(p/'decode_literals.py'))
b,pe,base=n['b'],n['pe'],n['base'];rows=[]
names=['ENUM_CMDPACKET_CHECK_DOUBLE_CHARACTER_NAME','ENUM_CMDPACKET_CREATE_CHARACTER','ENUM_CMDPACKET_SELECT_CHARACTER','ENUM_NOTIPACKET_CHARACTER_LIST','ENUM_NOTIPACKET_USER_INFO','ENUM_NOTIPACKET_LOGIN','ENUM_CMDPACKET_LOGIN']
for name in names:
 raw=name.encode('utf-16le');hits=[]
 for x in range(0,256,2):
  key=x|0x9a714ca0;enc=bytearray()
  for j in range(0,32,4):
   v=int.from_bytes(raw[j:j+4],'little');enc+=(v^key).to_bytes(4,'little');key=(key*0x1003f+v)&0xffffffff
  pos=0
  while True:
   pos=b.find(enc,pos)
   if pos<0:break
   va=base+pe.get_rva_from_offset(pos-4)
   try:
    decoded=n['decode'](va,2)
    if decoded==name:hits.append(va)
   except Exception:pass
   pos+=1
 rows.append({'name':name,'string_vas':[hex(x) for x in hits],'xrefs':[]})
 print(name,[hex(x) for x in hits],flush=True)
# RIP-relative LEA is a precise static reference candidate; caller validation follows.
mapping={int(v,16):r for r in rows for v in r['string_vas']};s=pe.sections[0];start=s.PointerToRawData;end=start+s.SizeOfRawData
for prefix in [b'\x48\x8d',b'\x4c\x8d']:
 pos=start
 while True:
  pos=b.find(prefix,pos,end)
  if pos<0:break
  if b[pos+2]&0xc7==5:
   va=base+s.VirtualAddress+pos-start;dest=va+7+struct.unpack_from('<i',b,pos+3)[0]
   if dest in mapping:
    fn=base+n['table'][bisect.bisect_right(n['begins'],va-base)-1][0];mapping[dest]['xrefs'].append({'va':hex(va),'function':hex(fn),'bytes':b[pos:pos+7].hex()})
  pos+=1
(p/'encoded_protocol_names.json').write_text(json.dumps(rows,indent=2),encoding='utf-8');print(json.dumps(rows,indent=2))

import runpy,contextlib,io,pathlib,struct,json,re,sys
with contextlib.redirect_stdout(io.StringIO()):n=runpy.run_path(r'E:\codex\2026-09-10\zhe\work\dfo_probe_tools\decode_literals.py')
b,pe,decode=n['b'],n['pe'],n['decode'];base=pe.OPTIONAL_HEADER.ImageBase;sec=next(s for s in pe.sections if s.Name.rstrip(b'\0')==b'.text');start=sec.PointerToRawData;end=start+sec.SizeOfRawData;pos=start;rows={};calls=0;invalid=0
while True:
 pos=b.find(b'\xe8',pos,end)
 if pos<0:break
 callva=base+sec.VirtualAddress+pos-start;dest=callva+5+struct.unpack_from('<i',b,pos+1)[0]
 if dest in (0x146e8c7d0,0x146e8c7b0) and b[pos-7:pos-4]==b'\x48\x8d\x0d':
  calls+=1;addr=callva+struct.unpack_from('<i',b,pos-4)[0];width=2 if dest==0x146e8c7d0 else 1
  if (addr,width) not in rows:
   try:
    s=decode(addr,width)
    if len(s)>32768 or any(0xd800<=ord(c)<=0xdfff for c in s):raise ValueError('surrogates')
    rows[(addr,width)]={'string_va':hex(addr),'call_va':hex(callva),'encoding':'utf-16le' if width==2 else 'utf-8','text':s}
   except Exception:invalid+=1
 pos+=1
out=pathlib.Path(r'E:\codex\2026-09-10\zhe\work\dfo_probe_tools');(out/'native_literals.json').write_text(json.dumps(list(rows.values()),ensure_ascii=False,indent=2),encoding='utf-8')
print('SCAN',json.dumps({'literal_calls':calls,'unique_decoded':len(rows),'invalid':invalid}))
hits=[r for r in rows.values() if re.search(r'Script\.pvf|DebugConfig|_DevConfig|single.?play|standalone|offline|pvf|argument parsing|CommandLine|SkipLogin',r['text'],re.I)]
(out/'startup_resource_literals.json').write_text(json.dumps(hits,ensure_ascii=False,indent=2),encoding='utf-8')
for r in hits[:45]:print(json.dumps(r,ensure_ascii=True))

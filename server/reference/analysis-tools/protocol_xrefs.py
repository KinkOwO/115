"""Exact-build static references, with bounded surrounding disassembly."""
import contextlib,io,runpy,pathlib,sys,bisect,struct,json,re
sys.stdout.reconfigure(encoding='utf-8')
p=pathlib.Path(__file__).parent
with contextlib.redirect_stdout(io.StringIO()): n=runpy.run_path(str(p/'decode_literals.py'))
pe,b,base,md,table,begins=[n[k] for k in ['pe','b','base','md','table','begins']]
results=[]
for arg in sys.argv[1:]:
 target=int(arg,16)
 refs=[]
 for s in pe.sections:
  if not s.Characteristics&0x20000000: continue
  src=s.get_data();va=base+s.VirtualAddress
  for match in re.finditer(b'[\xe8\xe9]',src):
   off=match.start()
   if off+5>len(src):continue
   if va+off+5+struct.unpack_from('<i',src,off+1)[0]!=target:continue
   addr=va+off
   beg,end,_=table[bisect.bisect_right(begins,addr-base)-1]
   refs.append({'at':hex(addr),'function':hex(base+beg),'kind':'call' if src[off]==0xe8 else 'jump'})
  for match in re.finditer(b'[\x48\x4c]\x8d[\x05\x0d\x15\x1d\x25\x2d\x35\x3d]',src):
   off=match.start()
   if off+7>len(src) or va+off+7+struct.unpack_from('<i',src,off+3)[0]!=target:continue
   addr=va+off;beg,end,_=table[bisect.bisect_right(begins,addr-base)-1]
   refs.append({'at':hex(addr),'function':hex(base+beg),'kind':'lea'})
 # Full absolute function pointers in data, including vtables and registrations.
 needle=struct.pack('<Q',target);off=0
 while True:
  off=b.find(needle,off)
  if off<0:break
  try:refs.append({'at':hex(base+pe.get_rva_from_offset(off)),'kind':'pointer'})
  except Exception:pass
  off+=1
 results.append({'target':hex(target),'references':refs})
print(json.dumps(results,ensure_ascii=False,indent=2))
(p/'latest_xrefs.json').write_text(json.dumps(results,indent=2),encoding='utf-8')

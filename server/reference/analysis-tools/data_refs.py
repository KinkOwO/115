"""Exact PE RIP-relative mov/lea references to specified data addresses."""
import runpy,contextlib,io,pathlib,struct,re,bisect,sys,json
p=pathlib.Path(__file__).parent
with contextlib.redirect_stdout(io.StringIO()):e=runpy.run_path(str(p/'decode_literals.py'))
targets={int(a,0) for a in sys.argv[2:]};rows=[]
for section in e['pe'].sections:
 if not section.Characteristics&0x20000000:continue
 b=section.get_data();base=e['base']+section.VirtualAddress
 for m in re.finditer(b'[\x48\x4c][\x8b\x8d\x89][\x05\x0d\x15\x1d\x25\x2d\x35\x3d]',b):
  k=m.start()
  if k+7>len(b):continue
  target=base+k+7+struct.unpack_from('<i',b,k+3)[0]
  if target not in targets:continue
  ix=bisect.bisect_right(e['begins'],base+k-e['base'])-1;a,z,_=e['table'][ix]
  if not a<=base+k-e['base']<z:continue
  rows.append({'target':hex(target),'at':hex(base+k),'function':hex(a+e['base'])})
(p/sys.argv[1]).write_text(json.dumps(rows,indent=2));print(json.dumps(rows,indent=2))

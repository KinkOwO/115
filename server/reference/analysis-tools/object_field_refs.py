"""Bounded static references to a supplied object displacement, not a live scan."""
import pathlib, contextlib, io, runpy, sys, struct, re, bisect, json
p=pathlib.Path(__file__).parent
with contextlib.redirect_stdout(io.StringIO()): n=runpy.run_path(str(p/'decode_literals.py'))
pe,md,base,table,begins=(n[k] for k in ('pe','md','base','table','begins'))
target=int(sys.argv[1],16)
out=[]; count=0; funcs=set()
for section in pe.sections:
 if not section.Characteristics & 0x20000000: continue
 data=section.get_data(); va=base+section.VirtualAddress
 for hit in re.finditer(re.escape(struct.pack('<I',target)),data):
  at=va+hit.start(); pos=bisect.bisect_right(begins,at-base)-1
  if pos<0:continue
  lo,hi,_=table[pos]
  if at-base>=hi or at-(base+lo)>0x20000:continue
  code=list(md.disasm(pe.get_data(lo,at-(base+lo)+20),base+lo))
  for i,x in enumerate(code):
   if not x.address<=at<x.address+x.size:continue
   if '[' not in x.op_str or hex(target) not in x.op_str:continue
   count+=1;funcs.add(hex(base+lo))
   if count<=120:
    out.append(f'FIELD {target:x} FUNCTION {base+lo:x}')
    out.extend(f'{v.address:x} {v.mnemonic} {v.op_str}' for v in code[max(0,i-3):i+5])
dest=p/f'object_field_{target:x}.asm';dest.write_text('\n'.join(out))
print(json.dumps(dict(field=hex(target),matches=count,functions=sorted(funcs),output=str(dest))))

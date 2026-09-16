"""Find exact native reads/writes of the inventory-ready member, read-only."""
import contextlib, io, pathlib, runpy
p=pathlib.Path(__file__).parent
with contextlib.redirect_stdout(io.StringIO()):
 e=runpy.run_path(str(p/'decode_literals.py'))
pe,md,base=e['pe'],e['md'],e['base']
from capstone.x86_const import X86_OP_MEM
rows=[]
for start,end,_ in e['table']:
 data=pe.get_data(start,end-start)
 if b'\x69\x14\x00\x00' not in data:continue
 ins=list(md.disasm(data,base+start))
 for i,x in enumerate(ins):
  if any(o.type==X86_OP_MEM and o.mem.disp==0x1469 for o in x.operands):
   rows.append(f'FUNCTION {base+start:x}')
   rows.extend(f'{v.address:x} {v.mnemonic} {v.op_str}' for v in ins[max(0,i-4):i+5])
for section in pe.sections:
 if not section.Characteristics&0x20000000:continue
 data=section.get_data(); pos=0
 while True:
  pos=data.find(b'\x69\x14\x00\x00',pos)
  if pos<0:break
  for back in range(2,5):
   off=pos-back
   ins=list(md.disasm(data[off:pos+16],base+section.VirtualAddress+off))
   if ins and any(o.type==X86_OP_MEM and o.mem.disp==0x1469 for o in ins[0].operands):
    rows.append('RAW '+ '; '.join(f'{v.address:x} {v.mnemonic} {v.op_str}' for v in ins[:4]))
  pos+=4
(p/'game_ready_refs.asm').write_text('\n'.join(rows)+'\n')
print('\n'.join(rows))

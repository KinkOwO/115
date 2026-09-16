"""Bounded native member consumers in character-card UI and info getters."""
import contextlib,io,pathlib,runpy
p=pathlib.Path(__file__).parent
with contextlib.redirect_stdout(io.StringIO()):e=runpy.run_path(str(p/'decode_literals.py'))
pe,md,base=e['pe'],e['md'],e['base']
from capstone.x86_const import X86_OP_MEM
fields={0xe28,0xe34,0x11e8,0x11f4,0x3560,0x3568}
rows=[]
for start,end,_ in e['table']:
 va=base+start
 if not 0x142ac0000<=va<0x142adffff:continue
 ins=list(md.disasm(pe.get_data(start,end-start),va))
 for i,x in enumerate(ins):
  if any(o.type==X86_OP_MEM and o.mem.disp in fields for o in x.operands):
   rows.append(f'FUNCTION {va:x}')
   rows.extend(f'{v.address:x} {v.mnemonic} {v.op_str}' for v in ins[max(0,i-5):i+6])
(p/'roster_model_refs.asm').write_text('\n'.join(rows)+'\n')
print('\n'.join(dict.fromkeys(v for v in rows if v.startswith('FUNCTION'))))

"""Read-only bounded search for native world return-map assignments."""
import contextlib, io, pathlib, runpy
p=pathlib.Path(__file__).parent
with contextlib.redirect_stdout(io.StringIO()):
 e=runpy.run_path(str(p/'decode_literals.py'))
pe,md,base=e['pe'],e['md'],e['base']
from capstone.x86_const import X86_OP_MEM
rows=[]
for start,end,_ in e['table']:
 va=base+start
 if not 0x146d00000<=va<0x146d17000:continue
 for x in md.disasm(pe.get_data(start,end-start),va):
  if x.mnemonic.startswith(('mov','cmp','lea')) and x.operands and x.operands[0].type==X86_OP_MEM and x.operands[0].mem.disp in (0x2a4,0x2a8):
   rows.append(f'{va:x} {x.address:x} {x.mnemonic} {x.op_str}')
print('\n'.join(rows))

"""Filter native scalar uses, excluding the many unrelated pointer members."""
import contextlib,io,pathlib,runpy,struct
p=pathlib.Path(__file__).parent
with contextlib.redirect_stdout(io.StringIO()):e=runpy.run_path(str(p/'decode_literals.py'))
from capstone.x86_const import X86_OP_MEM
fields={0x6d8,0x6dc,0x758,0x75c}
patterns=[struct.pack('<I',v) for v in fields];rows=[]
for start,end,_ in e['table']:
 data=e['pe'].get_data(start,end-start)
 if not any(v in data for v in patterns):continue
 ins=list(e['md'].disasm(data,e['base']+start))
 for i,x in enumerate(ins):
  if any(o.type==X86_OP_MEM and o.size==4 and o.mem.disp in fields for o in x.operands):
   rows.append(f'FUNCTION {e["base"]+start:x}')
   rows.extend(f'{v.address:x} {v.mnemonic} {v.op_str}' for v in ins[max(0,i-4):i+5])
(p/'character_scalar_consumers.asm').write_text('\n'.join(rows)+'\n')
print('lines',len(rows))

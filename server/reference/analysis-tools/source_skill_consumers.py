"""Locate native readers of the two character-template skill vectors."""
import contextlib,io,pathlib,runpy,struct
p=pathlib.Path(__file__).parent
with contextlib.redirect_stdout(io.StringIO()):e=runpy.run_path(str(p/'decode_literals.py'))
from capstone.x86_const import X86_OP_MEM,X86_REG_RSP,X86_REG_RBP
fields={0x4b0,0x558}; patterns=[struct.pack('<I',x) for x in fields]; rows=[]
for start,end,_ in e['table']:
 data=e['pe'].get_data(start,end-start)
 if not any(x in data for x in patterns):continue
 ins=list(e['md'].disasm(data,e['base']+start))
 hits=[]
 for i,x in enumerate(ins):
  if any(o.type==X86_OP_MEM and o.mem.disp in fields and o.mem.base not in (X86_REG_RSP,X86_REG_RBP) for o in x.operands):
   hits.extend(f'{a.address:x} {a.mnemonic} {a.op_str}' for a in ins[max(0,i-3):i+5])
 if hits:rows.append(f'FUNCTION {e["base"]+start:x}');rows.extend(hits)
(p/'source_skill_consumers.asm').write_text('\n'.join(rows)+'\n')
print('source skill vector candidates',len(rows))

"""Bounded static search for current spawn-record consumers, original EXE read only."""
import contextlib,io,pathlib,runpy
p=pathlib.Path(__file__).parent
with contextlib.redirect_stdout(io.StringIO()):e=runpy.run_path(str(p/'decode_literals.py'))
from capstone.x86_const import X86_OP_MEM,X86_REG_RSP,X86_REG_RBP
lines=[]
for start,end,_ in e['table']:
 if not 0x5b00000<=start<0x6080000 or end-start>0x8000:continue
 data=e['pe'].get_data(start,end-start)
 if not all(bytes([v]) in data for v in [0xc,0x10,0x14,0x40]):continue
 ins=list(e['md'].disasm(data,e['base']+start));regs={}
 for x in ins:
  for o in x.operands:
   if o.type==X86_OP_MEM and o.mem.base not in (X86_REG_RSP,X86_REG_RBP) and o.mem.disp in (0xc,0x10,0x14,0x40):regs.setdefault(o.mem.base,set()).add(o.mem.disp)
 hit={k for k,v in regs.items() if len(v)==4}
 if not hit:continue
 lines.append(f'FUNCTION {e["base"]+start:x} END {e["base"]+end:x}')
 for i,x in enumerate(ins):
  if any(o.type==X86_OP_MEM and o.mem.base in hit and o.mem.disp==0x40 for o in x.operands):lines.extend(f'{a.address:x} {a.mnemonic} {a.op_str}' for a in ins[max(0,i-3):i+8])
(p/'spawn_consumer_candidates.asm').write_text('\n'.join(lines),encoding='utf-8');print('candidate lines',len(lines))

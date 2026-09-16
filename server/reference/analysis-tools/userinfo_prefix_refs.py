"""Resolve current-build fixed-prefix accesses; no client process writes."""
import pathlib,contextlib,io,runpy,sys
sys.path.insert(0,str(pathlib.Path(__file__).parent/'python_lib'))
from capstone.x86 import X86_OP_MEM, X86_REG_RIP
p=pathlib.Path(__file__).parent
with contextlib.redirect_stdout(io.StringIO()):
 n=runpy.run_path(str(p/'decode_literals.py'))
pe,md,base=(n[k] for k in ('pe','md','base'))
regions=((0x14dc67340,160,'basic'),(0x14e66f260,250,'addition'))
out=[]
for lo,hi in ((0x145639400,0x145641b50),):
 ins=list(md.disasm(pe.get_data(lo-base,hi-lo),lo))
 for i,x in enumerate(ins):
  for op in x.operands:
   if op.type!=X86_OP_MEM or op.mem.base!=X86_REG_RIP:continue
   target=x.address+x.size+op.mem.disp
   for start,size,name in regions:
    if start<=target<start+size:
     out.append(f'{name}+{target-start:#x} size={op.size} at={x.address:x}')
     out.extend(f'{a.address:x} {a.mnemonic} {a.op_str}' for a in ins[max(0,i-1):i+4])
(p/'userinfo_prefix_refs.asm').write_text('\n'.join(out))
print('\n'.join(out))

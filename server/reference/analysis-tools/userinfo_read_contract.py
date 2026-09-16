"""Read-only packet reader contexts for current-build world entry."""
import pathlib,contextlib,io,runpy,bisect,json
p=pathlib.Path(__file__).parent
with contextlib.redirect_stdout(io.StringIO()): n=runpy.run_path(str(p/'decode_literals.py'))
pe,md,base,table,begins=(n[k] for k in ('pe','md','base','table','begins'))
readers={0x146ea1920:'u16',0x146ea0ba0:'u32',0x146ea09f0:'u8',0x146ea0be0:'raw',0x146d78070:'string',0x146d77f50:'compound'}
def code(fn,end=None):
 if end is None:
  a,b,_=table[bisect.bisect_right(begins,fn-base)-1];end=base+b
 return list(md.disasm(pe.get_data(fn-base,end-fn),fn)) if end>fn else []
def direct_reads(fn):
 return [(hex(x.address),readers[int(x.op_str,16)]) for x in code(fn) if x.mnemonic in ('call','jmp') and x.op_str.startswith('0x') and int(x.op_str,16) in readers]
lines=[];rows=[]
for fn,end in ((0x14563ec60,0x145641030),(0x14563d400,0x14563e280)):
 ins=code(fn,end);lines.append(f'FUNCTION {fn:x}')
 for i,x in enumerate(ins):
  if x.mnemonic not in ('call','jmp') or not x.op_str.startswith('0x'):continue
  target=int(x.op_str,16)
  if fn<=target<end:continue
  nested=direct_reads(target) if target not in readers and 0x145630000<=target<0x145650000 else []
  if target not in readers and not nested:continue
  row={'function':hex(fn),'at':hex(x.address),'target':hex(target),'reader':readers.get(target),'nested_reads':nested}
  rows.append(row);lines.append(json.dumps(row))
  lines.extend(f'{a.address:x} {a.mnemonic} {a.op_str}' for a in ins[max(0,i-4):i+3])
(p/'userinfo_read_contract.json').write_text(json.dumps(rows,indent=2))
(p/'userinfo_read_contract.asm').write_text('\n'.join(lines))
print('read sites/helpers',len(rows))

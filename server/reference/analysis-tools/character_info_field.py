"""Read-only references immediately following the exact CharacterInfo getter."""
import pathlib,json,runpy,contextlib,io
p=pathlib.Path(__file__).parent
with contextlib.redirect_stdout(io.StringIO()):e=runpy.run_path(str(p/'decode_literals.py'))
refs=json.loads((p/'latest_xrefs.json').read_text())[0]
assert refs['target']=='0x140176170'
out=[]
for r in refs['references']:
 if r['kind']!='call':continue
 a=int(r['at'],16)+5
 ins=list(e['md'].disasm(e['pe'].get_data(a-e['base'],90),a))
 regs={'rax'};hit=False
 for i,x in enumerate(ins[:12]):
  if any(o.type==3 and o.mem.disp==0x2c and x.reg_name(o.mem.base) in regs for o in x.operands):hit=True;break
  if x.mnemonic=='mov' and len(x.operands)==2 and all(o.type==1 for o in x.operands) and x.reg_name(x.operands[1].reg) in regs:regs.add(x.reg_name(x.operands[0].reg))
  if x.mnemonic in ('call','ret'):break
 if hit:
  out.append('FUNCTION '+r['function']+' GETTER '+r['at'])
  out.extend(f'{x.address:x} {x.mnemonic} {x.op_str}' for x in ins[:min(i+7,len(ins))])
(p/'character_info_field2c.asm').write_text('\n'.join(out)+'\n')
print('field2c references',sum(x.startswith('FUNCTION') for x in out))

"""Bounded candidate search for the native four-way skill grouping helper."""
import pathlib,runpy,contextlib,io
p=pathlib.Path(__file__).parent
with contextlib.redirect_stdout(io.StringIO()):e=runpy.run_path(str(p/'decode_literals.py'))
out=[];found=[]
for a,z,_ in e['table']:
 if not 0x5e00000<=a<0x6000000 or not 20<=z-a<=800:continue
 b=e['pe'].get_data(a,z-a)
 if b'\xb8\x03\x00\x00\x00' not in b or b'\xb8\x02\x00\x00\x00' not in b:continue
 ins=list(e['md'].disasm(b,a+e['base']))
 if not any(x.mnemonic=='cmp' and x.op_str.endswith(', 4') for x in ins):continue
 found.append(hex(a+e['base']));out.append('FUNCTION '+hex(a+e['base']))
 out.extend(f'{x.address:x} {x.mnemonic} {x.op_str}' for x in ins)
(p/'skill_group_candidates28.asm').write_text('\n'.join(out));print('group candidates',found)

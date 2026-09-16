"""Read only, bounded pdata function ranges containing explicit addresses."""
import contextlib,io,runpy,pathlib,sys,bisect
p=pathlib.Path(__file__).parent
with contextlib.redirect_stdout(io.StringIO()):e=runpy.run_path(str(p/'decode_literals.py'))
lines=[]
for arg in sys.argv[2:]:
 a=int(arg,0); i=bisect.bisect_right(e['begins'],a-e['base'])-1
 b,z,_=e['table'][i]
 if not b<=a-e['base']<z:
  b=a-e['base'];z=b+256
  decoded=list(e['md'].disasm(e['pe'].get_data(b,z-b),a))
  tail=next((x.address+x.size-e['base'] for i,x in enumerate(decoded) if x.mnemonic=='ret' or (i<3 and x.mnemonic=='jmp')),None)
  if tail is None:raise ValueError('leaf has no bounded return '+arg)
  z=tail
 if z-b>0x10000:raise ValueError(arg)
 lines.append(f'ADDRESS {a:x} FUNCTION {e["base"]+b:x} END {e["base"]+z:x}')
 lines.extend(f'{x.address:x} {x.mnemonic} {x.op_str}' for x in e['md'].disasm(e['pe'].get_data(b,z-b),e['base']+b))
(p/sys.argv[1]).write_text('\n'.join(lines),encoding='utf-8')
print(sys.argv[1],len(lines))

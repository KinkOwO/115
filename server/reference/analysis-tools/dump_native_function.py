"""Bounded static function dump, source EXE read-only."""
import contextlib,io,runpy,pathlib,sys,bisect
p=pathlib.Path(__file__).parent
with contextlib.redirect_stdout(io.StringIO()):e=runpy.run_path(str(p/'decode_literals.py'))
out=[]
for arg in sys.argv[2:]:
 start=int(arg,0);ix=bisect.bisect_right(e['begins'],start-e['base'])-1
 begin,end,_=e['table'][ix]
 if start-e['base']!=begin:raise ValueError('need exact pdata function start')
 if end-begin>0x10000:raise ValueError('function too large')
 out.append(f'FUNCTION {start:x} SIZE {end-begin:x}')
 out.extend(f'{x.address:x} {x.mnemonic} {x.op_str}' for x in e['md'].disasm(e['pe'].get_data(begin,end-begin),start))
target=p/sys.argv[1];target.write_text('\n'.join(out),encoding='utf-8')
print(target.name,'lines',len(out))

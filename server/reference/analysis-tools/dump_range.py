"""Bounded read-only disassembly of an explicit address range."""
import contextlib,io,pathlib,runpy,sys
p=pathlib.Path(__file__).parent
with contextlib.redirect_stdout(io.StringIO()):e=runpy.run_path(str(p/'decode_literals.py'))
a,z=map(lambda x:int(x,0),sys.argv[2:4])
if not 0<z-a<=0x8000:raise ValueError('range too large')
lines=[f'RANGE {a:x} {z:x}']+[f'{x.address:x} {x.mnemonic} {x.op_str}' for x in e['md'].disasm(e['pe'].get_data(a-e['base'],z-a),a)]
(p/sys.argv[1]).write_text('\n'.join(lines),encoding='utf-8');print(sys.argv[1],len(lines))

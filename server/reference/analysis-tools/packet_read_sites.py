"""Bounded exact-build packet-reader call contexts for an explicit region."""
import pathlib,contextlib,io,runpy,sys
p=pathlib.Path(__file__).parent
with contextlib.redirect_stdout(io.StringIO()):n=runpy.run_path(str(p/'decode_literals.py'))
pe,md,base=(n[k] for k in ('pe','md','base'))
readers={0x146ea09f0:'u8',0x146ea1920:'u16',0x146ea0ba0:'u32',0x146d77f90:'u64',0x146ea0be0:'raw',0x146d78070:'string',0x146d77f50:'sizedraw'}
start=int(sys.argv[1],16);size=int(sys.argv[2],16)
if not 0<size<=0x20000:raise ValueError('region bound')
ins=list(md.disasm(pe.get_data(start-base,size),start));out=[]
for i,x in enumerate(ins):
 if x.mnemonic not in ('call','jmp') or not x.op_str.startswith('0x') or int(x.op_str,16) not in readers:continue
 out.append('READ '+readers[int(x.op_str,16)])
 out.extend(f'{a.address:x} {a.mnemonic} {a.op_str}' for a in ins[max(0,i-5):i+8])
dest=p/f'packet_reads_{start:x}.asm';dest.write_text('\n'.join(out));print('\n'.join(out))

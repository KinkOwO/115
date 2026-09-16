import contextlib,io,runpy,json,pathlib,bisect,sys
sys.stdout.reconfigure(encoding='utf-8')
out=pathlib.Path(__file__).parent
with contextlib.redirect_stdout(io.StringIO()):
 n=runpy.run_path(str(out/'decode_literals.py'))
pe,md,decode,base,table,begins=[n[k] for k in ['pe','md','decode','base','table','begins']]
lines=[]
for spec in (sys.argv[1:] or ['144ecfbf0','146fe0520']):
 fn=int(spec.split('+')[0],16)
 begin,end,_=table[bisect.bisect_right(begins,fn-base)-1]
 if '+' in spec: end=fn-base+int(spec.split('+')[1],16)
 elif end<=fn-base: end=fn-base+0x100
 lines.append(f'FUNCTION {fn:X} RANGE {begin+base:X} {end+base:X}')
 ins=list(md.disasm(pe.get_data(fn-base,end-(fn-base)),fn))
 for i,x in enumerate(ins):
  line=f'{x.address:016X} {x.mnemonic} {x.op_str}'
  if x.mnemonic=='call' and x.op_str in ['0x146e8c7d0','0x146e8c7b0'] and i:
   p=ins[i-1]
   if p.mnemonic=='lea' and p.op_str.startswith('rcx, [rip'):
    try:line+=' ; '+decode(p.address+p.size+p.operands[1].mem.disp,2 if x.op_str.endswith('7d0') else 1).replace('\n','\\n')
    except Exception as e:line+=' ; ERROR '+str(e)
  lines.append(line)
(out/'startup_mode.asm').write_text('\n'.join(lines),encoding='utf-8')
print('\n'.join(x for x in lines if ' ; ' in x or x.startswith('FUNCTION')))

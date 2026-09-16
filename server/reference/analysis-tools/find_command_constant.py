import contextlib,io,runpy,pathlib,re,struct,sys,bisect
p=pathlib.Path(__file__).parent
with contextlib.redirect_stdout(io.StringIO()): n=runpy.run_path(str(p/'decode_literals.py'))
pe,base,md,table,begins=[n[k] for k in ['pe','base','md','table','begins']]
lines=[]
for arg in sys.argv[1:]:
    ident=int(arg); pattern=b'\xba'+struct.pack('<I',ident)
    for s in pe.sections:
        if not s.Characteristics&0x20000000: continue
        data=s.get_data(); va=base+s.VirtualAddress
        for hit in re.finditer(re.escape(pattern),data):
            addr=va+hit.start(); begin,_,_=table[bisect.bisect_right(begins,addr-base)-1]
            if addr-base-begin>0x10000: continue
            code=list(md.disasm(pe.get_data(begin,addr-base-begin+35),base+begin))
            k=next((i for i,x in enumerate(code) if x.address==addr),None)
            if k is None: continue
            lines.append(f'COMMAND {ident} at {addr:x}')
            lines.extend(f'{x.address:x} {x.mnemonic} {x.op_str}' for x in code[max(0,k-5):k+8])
(p/'command_constant_refs.asm').write_text('\n'.join(lines))
print('\n'.join(lines))

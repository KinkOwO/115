"""Locate current command registrations without executing the client."""
import contextlib,io,runpy,pathlib,re,struct,sys,json,bisect
p=pathlib.Path(__file__).parent
with contextlib.redirect_stdout(io.StringIO()): n=runpy.run_path(str(p/'decode_literals.py'))
pe,base,md,table,begins=[n[k] for k in ['pe','base','md','table','begins']]
targets={int(x) for x in sys.argv[1:]}
registrars={0x1459a2fb0,0x1459a3dd0,0x14599d450,0x14599d5d0}
registrations=[]
for s in pe.sections:
    if not s.Characteristics&0x20000000: continue
    b=s.get_data(); va=base+s.VirtualAddress
    for hit in re.finditer(b'\xe8',b):
        off=hit.start()
        if off+5>len(b):continue
        registrar=va+off+5+struct.unpack_from('<i',b,off+1)[0]
        if registrar not in registrars: continue
        at=va+off
        begin,_,_=table[bisect.bisect_right(begins,at-base)-1]
        if at-(base+begin)>0x20000: continue
        code=list(md.disasm(pe.get_data(begin,at-(base+begin)),base+begin))[-12:]
        packet=None; handler=None
        for x in code:
            if x.mnemonic=='mov' and x.op_str.startswith('edx, ') and x.operands[1].type==2: packet=x.operands[1].imm
            if x.mnemonic=='lea' and x.op_str.startswith('edx, [r9 + '): packet=x.operands[1].mem.disp
            if x.mnemonic=='lea' and x.op_str.startswith('r8, [rip'): handler=x.address+x.size+x.operands[1].mem.disp
        if packet in targets:
            registrations.append({'id':packet,'handler':hex(handler) if handler else None,'registration':hex(at),'registrar':hex(registrar)})
(p/'selected_registrations.json').write_text(json.dumps(registrations,indent=2))
print(json.dumps(registrations,indent=2))

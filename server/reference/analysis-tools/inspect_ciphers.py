import contextlib,io,runpy,pathlib,struct,json,bisect
p=pathlib.Path(__file__).parent
with contextlib.redirect_stdout(io.StringIO()): n=runpy.run_path(str(p/'decode_literals.py'))
pe,base,md,table,begins=[n[k] for k in ['pe','base','md','table','begins']]
def data(va,size):return pe.get_data(va-base,size)
def u64(va):return struct.unpack('<Q',data(va,8))[0]
rows=[]
for idx,rva in enumerate(struct.unpack('<14I',data(0x146d8f2d0,56))):
 calls=[x for x in md.disasm(data(base+rva,32),base+rva) if x.mnemonic=='call']
 ctor=int(calls[-1].op_str,16)
 _,end,_=table[bisect.bisect_right(begins,ctor-base)-1]
 ins=list(md.disasm(data(ctor,end+base-ctor),ctor))
 vt=next(x.address+x.size+x.operands[1].mem.disp for x in ins if x.mnemonic=='lea' and x.op_str.startswith('rax, [rip'))
 col=u64(vt-8);typerva=struct.unpack_from('<I',data(col,24),12)[0]
 name=pe.get_data(typerva+16,200).split(b'\0')[0].decode('ascii','replace')
 methods=[hex(v) for v in struct.unpack('<9Q',data(vt,72))]
 rows.append(dict(index=idx,constructor=hex(ctor),vtable=hex(vt),name=name,methods=methods))
(p/'cipher_algorithms.json').write_text(json.dumps(rows,indent=2))
print(json.dumps(rows,indent=2))

"""Bounded exact instruction references to selected object offsets."""
import contextlib,io,runpy,pathlib,sys,re,struct,bisect,json
p=pathlib.Path(__file__).parent
with contextlib.redirect_stdout(io.StringIO()):e=runpy.run_path(str(p/'decode_literals.py'))
rows=[]
for off in [int(x,16) for x in sys.argv[1:]]:
 for sec in e['pe'].sections:
  if not sec.Characteristics&0x20000000:continue
  data=sec.get_data();va=e['base']+sec.VirtualAddress
  for m in re.finditer(re.escape(struct.pack('<I',off)),data):
   at=va+m.start();start,end,_=e['table'][bisect.bisect_right(e['begins'],at-e['base'])-1]
   if end-start>0x20000:continue
   ins=e['md'].disasm(e['pe'].get_data(start,end-start),e['base']+start)
   for x in ins:
    if x.address>at:break
    if x.address<=at<x.address+x.size and any(o.type==3 and o.mem.disp==off for o in x.operands):
     rows.append({'offset':hex(off),'function':hex(e['base']+start),'site':hex(x.address),'instruction':x.mnemonic+' '+x.op_str});break
(p/'field_references.json').write_text(json.dumps(rows,indent=2),encoding='utf-8')
print(json.dumps(rows[:100],indent=2));print('total',len(rows))

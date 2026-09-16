import pathlib,sys,json,contextlib,io,runpy,re,bisect
p=pathlib.Path(__file__).parent
with contextlib.redirect_stdout(io.StringIO()): e=runpy.run_path(str(p/'decode_literals.py'))
for sec in e['pe'].sections:
 if not sec.Characteristics&0x20000000:continue
 data=sec.get_data();va=e['base']+sec.VirtualAddress
 for h in re.finditer(re.escape(bytes.fromhex('4a150000')),data):
  at=va+h.start();i=bisect.bisect_right(e['begins'],at-e['base'])-1
  start,end,_=e['table'][i]
  if end-start>0x20000:continue
  ins=list(e['md'].disasm(e['pe'].get_data(start,end-start),e['base']+start))
  matches=[x for x in ins if x.address<=at<x.address+x.size]
  if matches:print('tag154a',hex(e['base']+start),[(hex(x.address),x.mnemonic,x.op_str) for x in matches])
data=json.loads((p.parent/'dfo-lan/runtime/equipment_audit.json').read_text(encoding='utf-8'))
def num(f,k,default=-1):
 a=f.get(k,[])
 return a[0].get('value',a[0].get('Value',default)) if a else default
out=[]
for row in data['rows']:
 f=row['Fields']
 if num(f,'[creation rate]')>0 and 0<num(f,'[grade]')<=6:out.append(row)
print('LOW_GRADE_POSITIVE_EQUIPMENT',len(out))
for r in out[:12]:print(json.dumps(r,ensure_ascii=False))

import pathlib,runpy,contextlib,io,re,bisect,json
p=pathlib.Path(__file__).parent
with contextlib.redirect_stdout(io.StringIO()):e=runpy.run_path(str(p/'decode_literals.py'))
rows=[]
for sec in e['pe'].sections:
 if not sec.Characteristics&0x20000000:continue
 data=sec.get_data();va=e['base']+sec.VirtualAddress
 for hit in re.finditer(b'[\xb8-\xbf]'+re.escape(b'\x2e\x00\x00\x00')+b'|\x8d[\x40-\x7f]'+re.escape(b'\x2e'),data):
  site=va+hit.start();code=list(e['md'].disasm(data[hit.start():hit.start()+100],site))
  calls=[(hex(x.address),x.op_str) for x in code if x.mnemonic=='call']
  if not any(t.startswith('0x146d7') for _,t in calls):continue
  begin,end,_=e['table'][bisect.bisect_right(e['begins'],site-e['base'])-1]
  rows.append(dict(site=hex(site),function=hex(e['base']+begin),end=hex(e['base']+end),calls=calls))
(p/'result_builders.json').write_text(json.dumps(rows,indent=2))
print('result builder matches',len(rows))
for row in rows[:12]:print(json.dumps(row))

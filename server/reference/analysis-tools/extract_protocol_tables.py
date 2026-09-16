import pathlib,contextlib,io,runpy,bisect,json,struct,re
p=pathlib.Path(__file__).parent
with contextlib.redirect_stdout(io.StringIO()):n=runpy.run_path(str(p/'decode_literals.py'))
pe,md,base=n['pe'],n['md'],n['base'];rows=[]
for fn in [0x140069bb0,0x140075000]:
 begin,end,_=n['table'][bisect.bisect_right(n['begins'],fn-base)-1];print('INIT',hex(begin+base),hex(end+base))
 rcx_va=None;decoded=None
 for x in md.disasm(pe.get_data(begin,end-begin),base+begin):
  if x.mnemonic=='lea' and x.op_str.startswith('rcx, [rip'):rcx_va=x.address+x.size+x.operands[1].mem.disp
  elif x.mnemonic=='call' and x.op_str=='0x146e8c7d0':
   try:decoded=(n['decode'](rcx_va,2),rcx_va,x.address)
   except Exception:decoded=None
  elif x.mnemonic=='mov' and x.op_str.startswith('qword ptr [rip') and x.op_str.endswith(', rax') and decoded:
   dest=x.address+x.size+x.operands[0].mem.disp
   if decoded[0].startswith('ENUM_'):
    family='command' if 'CMDPACKET' in decoded[0] else 'notification';table=0x14ef38f60 if family=='command' else 0x14ef334b0
    rows.append({'family':family,'id':(dest-table)//8,'name':decoded[0],'string_va':hex(decoded[1]),'decode_call':hex(decoded[2]),'table_store_va':hex(x.address),'table_slot_va':hex(dest)})
   decoded=None
(p/'protocol_names.json').write_text(json.dumps(rows,ensure_ascii=False,indent=2),encoding='utf-8');print('ROWS',len(rows))
for r in rows:
 if (r['id']<20 or re.search('DOUBLE_CHARACTER|CHARAC_LIST|CHARACTER_LIST|USER_INFO|SERVER|DUNGEON.*ENT|TOWN|MY_INFO',r['name'])):print(json.dumps(r,ensure_ascii=True))

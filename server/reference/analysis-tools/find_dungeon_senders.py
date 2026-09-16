"""Find exact-build packet-builder callers for selected dungeon commands."""
import contextlib,io,pathlib,runpy,re,struct,bisect,json,sys
p=pathlib.Path(__file__).parent
with contextlib.redirect_stdout(io.StringIO()):e=runpy.run_path(str(p/'decode_literals.py'))
rows=[]
commands={int(x,0) for x in sys.argv[1:]} or {39,37,45,46,627}
for sec in e['pe'].sections:
 if not sec.Characteristics&0x20000000:continue
 data=sec.get_data();va=e['base']+sec.VirtualAddress
 for h in re.finditer(b'\xe8',data):
  i=h.start()
  if i+5>len(data) or va+i+5+struct.unpack_from('<i',data,i+1)[0]!=0x146d746e0:continue
  at=va+i;start,end,_=e['table'][bisect.bisect_right(e['begins'],at-e['base'])-1]
  code=list(e['md'].disasm(e['pe'].get_data(start,at-(e['base']+start)),e['base']+start))[-10:]
  for ins in code:
   if ins.mnemonic=='mov' and ins.op_str.startswith('edx, ') and ins.operands[1].type==2 and ins.operands[1].imm in commands:
    rows.append(dict(id=ins.op_str,site=hex(at),function=hex(e['base']+start)))
(p/'dungeon_senders.json').write_text(json.dumps(rows,indent=2));print(json.dumps(rows,indent=2))

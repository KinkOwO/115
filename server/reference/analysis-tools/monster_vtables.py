import contextlib,io,pathlib,runpy,struct,json
p=pathlib.Path(__file__).parent
with contextlib.redirect_stdout(io.StringIO()):e=runpy.run_path(str(p/'decode_literals.py'))
out=[]
for start in (0x1465b992e+0x49b7622,0x1465b9b0e+0x49bd8f2):
 row={'vtable':hex(start)}
 for off in (0xa40,0xb48):row[hex(off)]=hex(struct.unpack('<Q',e['pe'].get_data(start+off-e['base'],8))[0])
 out.append(row)
print(json.dumps(out,indent=2));(p/'monster_vtables.json').write_text(json.dumps(out,indent=2))

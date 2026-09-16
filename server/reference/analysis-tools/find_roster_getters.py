import contextlib,io,pathlib,runpy,struct
p=pathlib.Path(__file__).parent
with contextlib.redirect_stdout(io.StringIO()):e=runpy.run_path(str(p/'decode_literals.py'))
for sec in e['pe'].sections:
 if not sec.Characteristics&0x20000000:continue
 data=sec.get_data()
 for field in [0x6d0,0x6d4,0x6d8,0x6dc,0x750,0x754,0x758,0x75c]:
  for prefix in [b'\x8b\x81',b'\x0f\xb7\x81']:
   pattern=prefix+struct.pack('<I',field)+b'\xc3'; off=0
   while True:
    off=data.find(pattern,off)
    if off<0:break
    print(hex(field),hex(e['base']+sec.VirtualAddress+off))
    off+=len(pattern)

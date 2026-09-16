import pathlib,sys,struct,json,bisect,re
sys.path.insert(0,r'E:\codex\2026-09-10\zhe\work\dfo_probe_tools\python_lib')
import pefile,capstone
root=pathlib.Path(r'F:\dnfop\DFO');out=pathlib.Path(r'E:\codex\2026-09-10\zhe\work\dfo_probe_tools');pe=pefile.PE(str(root/'DFO.exe'),fast_load=True);b=(root/'DFO.exe').read_bytes();base=pe.OPTIONAL_HEADER.ImageBase
pd=next(s for s in pe.sections if s.Name.rstrip(b'\0')==b'.pdata');table=list(struct.iter_unpack('<III',b[pd.PointerToRawData:pd.PointerToRawData+(pd.Misc_VirtualSize//12)*12]));begins=[x[0] for x in table]
def decode(va,width):
 off=pe.get_offset_from_rva(va-base);h=b[off:off+4]
 if (width==1 and h[0]==0xa1) or (width==2 and h[:2]==b'\xa1\x2c'):
  raw=b[off+4:off+4096];pos=next((i for i in range(0,len(raw)-width+1,width) if raw[i:i+width]==b'\0'*width),len(raw));return raw[:pos].decode('utf-16le' if width==2 else 'utf-8','replace')
 x=h[1]&0xfe;n=(((x<<7)|x)^int.from_bytes(h[2:4],'little'))*width
 if n<=0 or n>32768:raise ValueError('size')
 key=x|0x9a714ca0;data=bytearray();src=b[off+4:off+4+n]
 for i in range(0,n//4*4,4):
  v=int.from_bytes(src[i:i+4],'little')^key;data+=v.to_bytes(4,'little');key=(key*0x1003f+v)&0xffffffff
 for i in range(n//4*4,n):
  v=src[i]^(key&255);data.append(v);key=(key*0x107+v)&0xffffffff
 return bytes(data).decode('utf-16le' if width==2 else 'utf-8','strict').rstrip('\0')
md=capstone.Cs(capstone.CS_ARCH_X86,capstone.CS_MODE_64);md.detail=True;records=[];lines=[]
for fn in [0x146de1b30,0x146de4310,0x146eb9630,0x1459968c0]:
 ix=bisect.bisect_right(begins,fn-base)-1;begin,end,unwind=table[ix]
 if begin!=fn-base: end=fn-base+0x1800
 lines.append(f'FUNCTION {fn:X} end {base+end:X}')
 inst=list(md.disasm(pe.get_data(fn-base,min(end-(fn-base),0x18000)),fn))
 for i,x in enumerate(inst):
  line=f'{x.address:016X} {x.mnemonic} {x.op_str}'
  if x.mnemonic=='call' and x.op_str in ['0x146e8c7d0','0x146e8c7b0'] and i:
   prev=inst[i-1]
   if prev.mnemonic=='lea' and prev.op_str.startswith('rcx, [rip'):
    addr=prev.address+prev.size+prev.operands[1].mem.disp
    try:
     s=decode(addr,2 if x.op_str.endswith('7d0') else 1);records.append({'function':hex(fn),'call_va':hex(x.address),'string_va':hex(addr),'text':s});line+=' ; '+s.replace('\n','\\n')
    except Exception as e:line+=' ; DECODE_ERROR '+str(e)
  lines.append(line)
(out/'startup_annotated.asm').write_text('\n'.join(lines),encoding='utf-8');(out/'startup_literals.json').write_text(json.dumps(records,ensure_ascii=False,indent=2),encoding='utf-8')
print('ANNOTATED_FUNCTIONS',4,'LITERAL_RECORDS',len(records))
for r in records:
 if re.search('Anti|Script|fail|Init|NPK|PVF|Security|Error|error',r['text']):print(json.dumps(r,ensure_ascii=True))

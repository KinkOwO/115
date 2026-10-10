from pathlib import Path
import hashlib,json,pefile,capstone,argparse
root=Path(__file__).resolve().parent
parser=argparse.ArgumentParser()
parser.add_argument('--client',required=True,type=Path)
args=parser.parse_args()
exe=args.client
sha=hashlib.sha256(exe.read_bytes()).hexdigest()
assert sha=='515fefa1c71577a15f90513f860270320f3502bc492ad596f10cf176b91c21eb'
p=pefile.PE(str(exe),fast_load=True);base=p.OPTIONAL_HEADER.ImageBase
md=capstone.Cs(capstone.CS_ARCH_X86,capstone.CS_MODE_64)
spec=[('Cycle',0x5d1f3b0,9),('Expansion',0x5d1a250,7),('Resolve',0x5cdc8e0,5),('Lookup',0x5c48fe0,5),('Skin',0x5d71ad0,6),('Shape',0x5c20190,8)]
binding={'client_sha256':sha,'functions':{},'mechanics':['synthetic weapon actor+6048','eligible skin native getter145d71ad0','slot10 retained by native145d1f500 iteration','visual scopes only; no per-template content rules'],'attempt':'animation adapter 1/3, owner-authorized after live failure; prior visual attempts retained'}
out=['#pragma once','#include <stdint.h>',f'static const DWORD kImageTimestamp={p.FILE_HEADER.TimeDateStamp}u;',f'static const DWORD kImageSize={p.OPTIONAL_HEADER.SizeOfImage}u;']
for name,rva,size in spec:
 data=p.get_data(rva,size);ins=list(md.disasm(data,base+rva))
 assert sum(i.size for i in ins)==size
 if name not in ['Skin','Shape']:assert all(i.mnemonic not in ['jmp','call'] and 'rip' not in i.op_str for i in ins)
 binding['functions'][name]={'rva':hex(rva),'bytes':data.hex(),'instructions':[f'{i.mnemonic} {i.op_str}' for i in ins]}
 out += [f'static const uintptr_t k{name}RVA={hex(rva)};',f'static const unsigned char k{name}Head[]={{'+','.join(hex(b) for b in data)+'};']
(root/'dist').mkdir(exist_ok=True)
(root/'dist/native_evidence.h').write_text('\n'.join(out)+'\n','utf8')
loop=p.get_data(0x5d1f500,0x73)
assert hashlib.sha256(loop).hexdigest()=='60cb8a37ef827a36af67ce49615f5a4f3101a5996ed69060bad5f49ac2dccbb5'
(root/'dist/native_loop.h').write_text('static const unsigned char kNativeLoop[]={'+','.join(hex(b) for b in loop)+'};\n','utf8')
binding['native_loop_sha256']=hashlib.sha256(loop).hexdigest()
(root/'dist/source-binding.json').write_text(json.dumps(binding,indent=2)+'\n','utf8')
print(json.dumps(binding,indent=2))

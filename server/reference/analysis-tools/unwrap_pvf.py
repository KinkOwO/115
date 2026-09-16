"""Read-only asset wrapper compatibility for the exact supplied DFO build.

Native loader 147C4D990 uses its embedded RSA key to read sk.dat, then AES-CBC
with zero IV on selected prefixes. Keys stay in memory and are never printed.
Output is separate from the original; this does not patch the game or PVF.
"""
import pathlib,sys,struct,hashlib,json
sys.path.insert(0,str(pathlib.Path(__file__).parent/'python_lib'))
import pefile
from cryptography.hazmat.primitives.serialization import load_pem_private_key
from cryptography.hazmat.primitives.asymmetric.padding import PKCS1v15
from cryptography.hazmat.primitives.ciphers import Cipher,algorithms,modes

root=pathlib.Path(r'F:/dnfop/DFO')
out=pathlib.Path(__file__).parent.parent/'dfo-lan/runtime/pvf_source'
out.mkdir(parents=True,exist_ok=True)
exe=root/'DFO.exe';pe=pefile.PE(str(exe),fast_load=True)
def string_at_pointer(va):
 ptr=struct.unpack('<Q',pe.get_data(va-0x140000000,8))[0]
 return pe.get_data(ptr-0x140000000,4096).split(b'\0')[0]
def cbc(key,raw):
 d=Cipher(algorithms.AES(key),modes.CBC(bytes(16))).decryptor()
 return d.update(raw)+d.finalize()
def stream(key,raw):
 k=[ord(x) for x in key];seed=(0x339e9711*k[0]+0x393*(k[3]+0x393*(k[2]+0x393*k[1])))&0xffffffff
 out=bytearray(raw)
 for off in range(0,len(out),4):
  t=(0x343fd*seed+0x269ec3)&0xffffffff;seed=(0x343fd*t+0x269ec3)&0xffffffff
  q=struct.pack('<I',(t&0xffff0000)+(seed>>16))
  for i in range(min(4,len(out)-off)):out[off+i]^=q[i]
 return bytes(out)

private=load_pem_private_key(string_at_pointer(0x14dc98110),password=None)
data=(root/'sk.dat').read_bytes();size=private.key_size//8
if len(data)%size:raise ValueError('RSA block alignment')
unwrapped=b''.join(private.decrypt(data[i:i+size],PKCS1v15()) for i in range(0,len(data),size))
prefix=len(unwrapped)//256*256
unwrapped=cbc(bytes.fromhex(string_at_pointer(0x14dc98118).decode()),unwrapped[:prefix])+unwrapped[prefix:]
if len(unwrapped)%32:raise ValueError('chunk key alignment')
keys=[unwrapped[i:i+32] for i in range(0,len(unwrapped),32)]
with (root/'Script.pvf').open('rb') as f:first=f.read(0x2800)
header=stream('iNfO',cbc(keys[0],first)[:48])
if header[:4]!=b'nkpi':raise ValueError('native archive header signature did not match')
print('INNER_HEADER_VALID',list(struct.unpack('<12I',header)),'keys',len(keys))
target=out/'Script.inner.pvf'
if target.exists():raise SystemExit('Output already exists; preserve it')
sourcehash=hashlib.sha256();innerhash=hashlib.sha256()
with (root/'Script.pvf').open('rb') as source,target.open('xb') as destination:
 index=0
 while chunk:=source.read(0xa00000):
  sourcehash.update(chunk)
  if index<len(keys):
   if len(chunk)<0x2800:raise ValueError('encrypted segment too short')
   chunk=cbc(keys[index],chunk[:0x2800])+chunk[0x2800:]
  destination.write(chunk);innerhash.update(chunk);index+=1
manifest={'source':str(root/'Script.pvf'),'source_sha256':sourcehash.hexdigest(),'output':str(target),'inner_sha256':innerhash.hexdigest(),'segments':index,'key_count':len(keys),'header_words':list(struct.unpack('<12I',header)),'native_loader':'0x147C4D990','header_stream':'0x147C4D810 / iNfO','status':'inner NKPI signature verified; catalog decoding pending'}
(out/'unwrap.json').write_text(json.dumps(manifest,indent=2),encoding='utf-8')
print(json.dumps({k:manifest[k] for k in ['source_sha256','inner_sha256','segments','output']}))

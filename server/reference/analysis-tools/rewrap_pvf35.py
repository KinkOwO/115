"""Reapply the exact existing asset wrapper to a separate UI-patched PVF.

No originals or live-client files are written. Embedded wrapper material is
read only in memory and is never logged or included in the output manifest.
"""
from pathlib import Path
import sys,struct,hashlib,json
sys.path.insert(0,str(Path(__file__).parent/'python_lib'))
import pefile
from cryptography.hazmat.primitives.serialization import load_pem_private_key
from cryptography.hazmat.primitives.asymmetric.padding import PKCS1v15
from cryptography.hazmat.primitives.ciphers import Cipher,algorithms,modes
root=Path('F:/dnfop/DFO');dest=Path(__file__).parent.parent/'dfo-lan/runtime/archer-layout-patch35'
source=dest/'Script.inner.pvf';output=dest/'Script.pvf'
if output.exists():raise ValueError('output already exists; preserve it')
pe=pefile.PE(str(root/'DFO.exe'),fast_load=True)
def literal(va):
 ptr=struct.unpack('<Q',pe.get_data(va-0x140000000,8))[0]
 return pe.get_data(ptr-0x140000000,4096).split(b'\0')[0]
def cbc(key,p,encrypt=False):
 cipher=Cipher(algorithms.AES(key),modes.CBC(bytes(16)))
 op=cipher.encryptor() if encrypt else cipher.decryptor()
 return op.update(p)+op.finalize()
private=load_pem_private_key(literal(0x14dc98110),password=None)
data=(root/'sk.dat').read_bytes();size=private.key_size//8
if len(data)%size:raise ValueError('wrapper alignment')
unwrapped=b''.join(private.decrypt(data[i:i+size],PKCS1v15()) for i in range(0,len(data),size))
prefix=len(unwrapped)//256*256
unwrapped=cbc(bytes.fromhex(literal(0x14dc98118).decode()),unwrapped[:prefix])+unwrapped[prefix:]
if len(unwrapped)%32:raise ValueError('wrapper key alignment')
keys=[unwrapped[i:i+32] for i in range(0,len(unwrapped),32)]
before=hashlib.sha256();after=hashlib.sha256();recovered=hashlib.sha256()
with source.open('rb') as src,output.open('xb') as dst:
 index=0
 while block:=src.read(0xa00000):
  before.update(block)
  if index<len(keys):
   if len(block)<0x2800:raise ValueError('short wrapped prefix')
   wrapped=cbc(keys[index],block[:0x2800],True)+block[0x2800:]
   recovered.update(cbc(keys[index],wrapped[:0x2800])+wrapped[0x2800:])
  else:wrapped=block;recovered.update(block)
  dst.write(wrapped);after.update(wrapped);index+=1
assert before.digest()==recovered.digest()
manifest={'input':str(source.resolve()),'output':str(output.resolve()),'inner_sha256':before.hexdigest(),'wrapped_sha256':after.hexdigest(),'native_wrapper_roundtrip':True,'segments':index,'original_files_modified':False}
(dest/'wrapper-manifest.json').write_text(json.dumps(manifest,indent=2))
print(json.dumps(manifest,indent=2))

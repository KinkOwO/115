"""Pure native AES object validation. No process, UI, network or entry point."""
from native_cipher_oracle import Oracle
import struct,json,pathlib
from cryptography.hazmat.primitives.ciphers import Cipher,algorithms,modes
o=Oracle();obj=0x220000;keyptr=0x210000;ivptr=0x211000
o.call(0x146e65070,obj)
vtable=struct.unpack('<Q',o.u.mem_read(obj,8))[0]
methods=struct.unpack('<8Q',o.pe.get_data(vtable-o.base,64))
print('METHODS',*[hex(x) for x in methods])
rows=[]
for seed in [0,1,0x47]:
 key=bytes((i+seed)&255 for i in range(16));plain=bytes((i*7+seed)&255 for i in range(32))
 o.u.mem_write(keyptr,key);o.u.mem_write(ivptr,b'\0'*16)
 status=o.call(0x146e66440,obj,keyptr,16,ivptr,16,16,0,0)
 if status!=0x6fffffff:raise RuntimeError(f'SetKey {status:x}')
 o.u.mem_write(0x230000,plain)
 encstatus=o.call(methods[2],obj,0x230000,0x240000,len(plain))
 cipher=bytes(o.u.mem_read(0x240000,len(plain)))
 decstatus=o.call(methods[3],obj,0x240000,0x250000,len(plain))
 recovered=bytes(o.u.mem_read(0x250000,len(plain)))
 reference=Cipher(algorithms.AES(key),modes.ECB()).encryptor().update(plain)
 print('RESULT',seed,hex(encstatus),hex(decstatus),cipher.hex(),'standard',cipher==reference,'roundtrip',recovered==plain)
 if recovered!=plain or cipher!=reference:raise RuntimeError('not standard AES ECB')
 rows.append(dict(algorithm='aes128',key=key.hex(),plain=plain.hex(),cipher=cipher.hex(),setup='0x146e66440',encrypt=hex(methods[2]),decrypt=hex(methods[3])))
pathlib.Path(__file__).with_name('native_aes_vectors.json').write_text(json.dumps(rows,indent=2))

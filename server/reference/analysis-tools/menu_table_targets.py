import pathlib, struct
from native_cipher_oracle import Oracle
o=Oracle()
for i in [0,5,10]:
 off=struct.unpack('<I',o.pe.get_data(0x1458dbbac-o.base+i*4,4))[0]
 print(f'inventory action {i}: {o.base+off:x}')

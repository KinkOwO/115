from pathlib import Path
import struct
b=(Path(__file__).parent.parent/'dfo-lan/runtime/options-source35/00-unifiedoption.ctp.bin').read_bytes()
for p in range(0,400,4):
 print(hex(p),struct.unpack_from('<I',b,p)[0])

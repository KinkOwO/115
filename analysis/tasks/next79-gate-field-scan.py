"""Read-only search for native roster qualification flag tests."""
import pefile
pe=pefile.PE(r"D:/115us/client/DFO.exe",fast_load=True)
base=pe.OPTIONAL_HEADER.ImageBase
start=0x24e0000
data=pe.get_data(start,0x50000)
for value in range(0x638,0x654):
    pat=value.to_bytes(4,"little")
    off=0
    while (off:=data.find(pat,off))>=0:
        pre=data[max(0,off-3):off]
        if b"\xf6" in pre or b"\xf7" in pre:
            print(hex(base+start+off),hex(value),data[off-3:off+7].hex())
        off+=4

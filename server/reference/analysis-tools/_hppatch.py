import sys, pathlib, hashlib, struct
sys.path.insert(0, str(pathlib.Path(__file__).parent/'python_lib'))
import pefile

client = pathlib.Path(r'E:\codex\2026-09-10\zhe\work\dfo_probe_client\DFO.exe')
retail = pathlib.Path(r'F:\dnfop\DFO\DFO.exe')
VA = 0x147220f48                 # jbe 0x147221000 in getter 147220e40
EXPECT = bytes([0x0F,0x86,0xB2,0x00,0x00,0x00])   # jbe rel32=0xB2 -> 147221000
# jmp near to same target: E9 rel32' 90 ; rel32' = 147221000 - (147220f48+5)
NEWB   = bytes([0xE9,0xB3,0x00,0x00,0x00,0x90])

def sha(p): return hashlib.sha256(p.read_bytes()).hexdigest()
print("client exists:", client.exists(), "retail exists:", retail.exists())
if client.exists():
    print("client sha:", sha(client)[:16])
if retail.exists():
    print("retail sha:", sha(retail)[:16])
    print("identical:", sha(client)==sha(retail) if client.exists() else "n/a")

pe = pefile.PE(str(client), fast_load=True)
ib = pe.OPTIONAL_HEADER.ImageBase
rva = VA - ib
foff = None
for s in pe.sections:
    if s.VirtualAddress <= rva < s.VirtualAddress + max(s.Misc_VirtualSize, s.SizeOfRawData):
        foff = rva - s.VirtualAddress + s.PointerToRawData
        sec = s.Name.rstrip(b'\x00').decode()
        break
print(f"ImageBase={ib:#x} RVA={rva:#x} section={sec} file_offset={foff:#x}")

data = bytearray(client.read_bytes())
cur = bytes(data[foff:foff+6])
print("bytes at site:", cur.hex(), " expected:", EXPECT.hex(), " match:", cur==EXPECT)
if cur != EXPECT:
    print("!! ABORT: bytes do not match; no patch written.")
    sys.exit(2)
data[foff:foff+6] = NEWB
out = client.parent/'DFO.exe.hppatched'
out.write_bytes(bytes(data))
print("patched copy written:", out, " new bytes:", NEWB.hex())
print("orig sha:", hashlib.sha256(client.read_bytes()).hexdigest()[:16])
print("patched sha:", hashlib.sha256(out.read_bytes()).hexdigest()[:16])
print("only 6 bytes differ:", sum(1 for a,b in zip(client.read_bytes(), out.read_bytes()) if a!=b)==6 and len(client.read_bytes())==len(out.read_bytes()))

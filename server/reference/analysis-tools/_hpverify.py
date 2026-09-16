import sys, pathlib, hashlib
sys.path.insert(0, str(pathlib.Path(__file__).parent/'python_lib'))
import pefile
client = pathlib.Path(r'E:\codex\2026-09-10\zhe\work\dfo_probe_client\DFO.exe')
retail = pathlib.Path(r'F:\dnfop\DFO\DFO.exe')
VA = 0x147220f48
EXPECT = bytes([0x0F,0x86,0xB2,0x00,0x00,0x00])
def sha(p): return hashlib.sha256(p.read_bytes()).hexdigest()
print("client exists:", client.exists(), "size:", client.stat().st_size if client.exists() else 0)
print("client sha:", sha(client)[:16] if client.exists() else "n/a")
print("retail sha :", sha(retail)[:16] if retail.exists() else "n/a")
print("client==retail:", (sha(client)==sha(retail)) if (client.exists() and retail.exists()) else "n/a")
pe = pefile.PE(str(client), fast_load=True)
ib = pe.OPTIONAL_HEADER.ImageBase
rva = VA - ib
foff=None; sec=None
for s in pe.sections:
    if s.VirtualAddress <= rva < s.VirtualAddress + max(s.Misc_VirtualSize, s.SizeOfRawData):
        foff = rva - s.VirtualAddress + s.PointerToRawData; sec=s.Name.rstrip(b'\x00').decode(); break
data = client.read_bytes()
cur = data[foff:foff+6]
print(f"ImageBase={ib:#x} RVA={rva:#x} section={sec} file_offset={foff:#x}")
print("bytes at site:", cur.hex(), "expected:", EXPECT.hex(), "MATCH:", cur==EXPECT)
print("context [-4..+10]:", data[foff-4:foff+10].hex())

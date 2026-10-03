"""Read-only local-client disassembly; avoids the different root DFO.exe."""
import struct
import sys
from pathlib import Path

import capstone
import pefile

root = Path(__file__).resolve().parents[2]
exe = root.parent / "client/DFO.exe"
pe = pefile.PE(str(exe), fast_load=True)
base = pe.OPTIONAL_HEADER.ImageBase
md = capstone.Cs(capstone.CS_ARCH_X86, capstone.CS_MODE_64)
if sys.argv[1] == "--packet":
    value = struct.pack("<I", int(sys.argv[2]))
    section = next(s for s in pe.sections if s.Name.startswith(b".text"))
    data = section.get_data()
    for prefix in (b"\xba", b"\xb9", b"\x41\xb8", b"\x41\xb9"):
        pattern = prefix + value
        off = 0
        while (off := data.find(pattern, off)) >= 0:
            print(hex(base + section.VirtualAddress + off), pattern.hex())
            off += len(pattern)
else:
    va = int(sys.argv[1], 16)
    size = int(sys.argv[2], 0) if len(sys.argv) > 2 else 256
    for ins in md.disasm(pe.get_data(va - base, size), va):
        print(f"{ins.address:#x} {ins.mnemonic:8} {ins.op_str}")

"""Read image constants by virtual address, and find every reference to a
float bit pattern.

    python read_image_float36.py 0x1491be9c8 ...
    python read_image_float36.py --find 3c6a4a8d

Read-only static inspection of the client image.
"""
import contextlib
import io
import pathlib
import runpy
import struct
import sys

p = pathlib.Path(__file__).parent
with contextlib.redirect_stdout(io.StringIO()):
    n = runpy.run_path(str(p / 'decode_literals.py'))
pe, base, md = n['pe'], n['base'], n['md']


def read(va, size):
    return pe.get_data(va - base, size)


if sys.argv[1] == '--find':
    want = struct.pack('<I', int(sys.argv[2], 16))
    for s in pe.sections:
        data = s.get_data()
        start = base + s.VirtualAddress
        name = s.Name.rstrip(b'\0').decode('latin1')
        off = data.find(want)
        hits = 0
        while off != -1 and hits < 40:
            print(f'{name} 0x{start + off:x}')
            hits += 1
            off = data.find(want, off + 1)
    raise SystemExit

for arg in sys.argv[1:]:
    va = int(arg, 0)
    raw = read(va, 8)
    u32 = struct.unpack_from('<I', raw)[0]
    print(f'{va:#x}  bytes={raw.hex()}  u32={u32:#010x}  '
          f'float={struct.unpack_from("<f", raw)[0]!r}  '
          f'double={struct.unpack_from("<d", raw)[0]!r}')

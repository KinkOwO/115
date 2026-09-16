"""Rebuild the exact required client artifacts from the pristine client.

Produces:
  DFO\\DFO_required_built.exe   (expect sha256 1d394878...)
  <workdir>\\Script.required.pvf (expect sha256 5dd03873..., 760531281 bytes)
Patches/algorithm copied verbatim from reference/analysis-tools.
"""
import hashlib, json, pathlib, struct, sys

import pefile
from cryptography.hazmat.primitives.serialization import load_pem_private_key
from cryptography.hazmat.primitives.asymmetric.padding import PKCS1v15
from cryptography.hazmat.primitives.ciphers import Cipher, algorithms, modes

CLIENT = pathlib.Path(r'D:\115us\client')
ORIG_EXE = CLIENT / 'DFO_original.exe'
WORK = pathlib.Path(r'D:\115us\server\work\client-build')
WORK.mkdir(parents=True, exist_ok=True)

REQUIRED_EXE_SHA = '1d3948784e5e0f77ed744017bf82bf0c50de9421423f9e59f6f70aed609d8ffa'
REQUIRED_PVF_SHA = '5dd03873edf2c1df7aea16db5ad146a776fb8461a947be73042cabd932e66f0a'
ORIG_SHA = 'fda6c33f2a155124a4262f3e7cd6f7a18d4e2191e5f147aec5754716afe5aa7c'

# ---------- EXE: key-pointer patch + HP patch ----------
def build_exe():
    b = bytearray(ORIG_EXE.read_bytes())
    base_sha = hashlib.sha256(b).hexdigest()
    assert base_sha == ORIG_SHA, base_sha
    pe = pefile.PE(data=bytes(b), fast_load=True)
    ib = pe.OPTIONAL_HEADER.ImageBase

    # 1) ChannelInfo ArenaStringPtr dereference before SetKeys (0x1452c803e)
    va, end = 0x1452C803E, 0x1452C805B
    o = pe.get_offset_from_rva(va - ib)
    old = bytes(b[o:o + end - va])
    assert old[:9] == bytes.fromhex('488D4B08E8F9169E01'), old.hex()
    patch = bytes.fromhex('4883E2FC488B12488D4B08')
    patch += b'\xE8' + struct.pack('<i', 0x146CA9740 - (va + len(patch) + 5))
    patch += b'\x90' * (len(old) - len(patch))
    b[o:o + len(patch)] = patch

    # 2) HP getter: jbe -> jmp (0x147220f48)
    va2 = 0x147220F48
    o2 = pe.get_offset_from_rva(va2 - ib)
    assert bytes(b[o2:o2 + 6]) == bytes.fromhex('0F86B2000000')
    b[o2:o2 + 6] = bytes.fromhex('E9B300000090')

    out = CLIENT / 'DFO_required_built.exe'
    out.write_bytes(bytes(b))
    sha = hashlib.sha256(bytes(b)).hexdigest()
    print('EXE', sha, 'MATCH' if sha == REQUIRED_EXE_SHA else 'MISMATCH')
    return sha == REQUIRED_EXE_SHA

# ---------- PVF unwrap / rewrap ----------
def load_material():
    pe = pefile.PE(str(ORIG_EXE), fast_load=True)

    def literal(va):
        ptr = struct.unpack('<Q', pe.get_data(va - 0x140000000, 8))[0]
        return pe.get_data(ptr - 0x140000000, 4096).split(b'\0')[0]

    def cbc(key, raw, encrypt=False):
        c = Cipher(algorithms.AES(key), modes.CBC(bytes(16)))
        op = c.encryptor() if encrypt else c.decryptor()
        return op.update(raw) + op.finalize()

    private = load_pem_private_key(literal(0x14DC98110), password=None)
    data = (CLIENT / 'sk.dat').read_bytes()
    size = private.key_size // 8
    unwrapped = b''.join(private.decrypt(data[i:i + size], PKCS1v15())
                         for i in range(0, len(data), size))
    prefix = len(unwrapped) // 256 * 256
    unwrapped = cbc(bytes.fromhex(literal(0x14DC98118).decode()),
                    unwrapped[:prefix]) + unwrapped[prefix:]
    keys = [unwrapped[i:i + 32] for i in range(0, len(unwrapped), 32)]
    return keys, cbc

def build_pvf():
    keys, cbc = load_material()
    inner = WORK / 'Script.inner.pvf'
    # unwrap
    ih = hashlib.sha256()
    with (CLIENT / 'Script.pvf').open('rb') as src, inner.open('wb') as dst:
        index = 0
        while chunk := src.read(0xA00000):
            if index < len(keys):
                chunk = cbc(keys[index], chunk[:0x2800]) + chunk[0x2800:]
            dst.write(chunk)
            ih.update(chunk)
            index += 1
    print('INNER', ih.hexdigest(), 'segments', index, 'keys', len(keys))

    # rewrap
    out = WORK / 'Script.pvf'
    if out.exists():
        out.unlink()
    wh = hashlib.sha256()
    with inner.open('rb') as src, out.open('wb') as dst:
        index = 0
        while block := src.read(0xA00000):
            if index < len(keys):
                wrapped = cbc(keys[index], block[:0x2800], True) + block[0x2800:]
            else:
                wrapped = block
            dst.write(wrapped)
            wh.update(wrapped)
            index += 1
    sha = wh.hexdigest()
    print('PVF', sha, out.stat().st_size,
          'MATCH' if sha == REQUIRED_PVF_SHA else 'MISMATCH')
    return sha == REQUIRED_PVF_SHA

if __name__ == '__main__':
    ok1 = build_exe()
    ok2 = build_pvf()
    sys.exit(0 if ok1 and ok2 else 1)

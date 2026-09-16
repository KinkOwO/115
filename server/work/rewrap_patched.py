"""Rewrap the patched inner PVF with the native outer layer and verify sha."""
import hashlib, pathlib, sys
sys.path.insert(0, r'D:\115us\server\work')
from build_required import load_material, REQUIRED_PVF_SHA

WORK = pathlib.Path(r'D:\115us\server\work\client-build')
keys, cbc = load_material()
src = WORK / 'Script.patched.inner.pvf'
out = WORK / 'Script.required.pvf'
if out.exists():
    out.unlink()
h = hashlib.sha256()
index = 0
with src.open('rb') as fi, out.open('wb') as fo:
    while block := fi.read(0xA00000):
        if index < len(keys):
            wrapped = cbc(keys[index], block[:0x2800], True) + block[0x2800:]
        else:
            wrapped = block
        fo.write(wrapped)
        h.update(wrapped)
        index += 1
sha = h.hexdigest()
print('PVF', sha, out.stat().st_size, 'segments', index)
print('MATCH' if sha == REQUIRED_PVF_SHA else 'MISMATCH')
sys.exit(0 if sha == REQUIRED_PVF_SHA else 1)

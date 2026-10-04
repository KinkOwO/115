"""只读解析当前 115 客户端 PVF，供强化券规则导出使用。"""
import argparse
from functools import lru_cache
from pathlib import Path
import struct, hashlib, re, zlib
import pefile
from cryptography.hazmat.primitives.serialization import load_pem_private_key
from cryptography.hazmat.primitives.asymmetric.padding import PKCS1v15
from cryptography.hazmat.primitives.ciphers import Cipher, algorithms, modes

def stream(key, raw):
    seed = (0x339e9711*ord(key[0])+0x393*(ord(key[3])+0x393*(ord(key[2])+0x393*ord(key[1])))) & 0xffffffff
    out = bytearray(raw)
    for offset in range(0, len(out), 4):
        first = (0x343fd*seed+0x269ec3) & 0xffffffff
        seed = (0x343fd*first+0x269ec3) & 0xffffffff
        mask = struct.pack('<I', (first & 0xffff0000)+(seed >> 16))
        for index in range(min(4, len(out)-offset)):
            out[offset+index] ^= mask[index]
    return out

def aes(key, raw, encrypt=False):
    cipher = Cipher(algorithms.AES(key), modes.CBC(bytes(16)))
    operation = cipher.encryptor() if encrypt else cipher.decryptor()
    return operation.update(raw)+operation.finalize()

_PEM_BLOCK = re.compile(b'-----BEGIN PRIVATE KEY-----.*?-----END PRIVATE KEY-----', re.S)
_AES_HEX = re.compile(rb'[0-9A-F]{64}(?![0-9A-F])')

def _fixed_va_literals(root):
    """2.38.2.34 布局：DFO.exe 固定 VA 槽位里的 (PEM, AES hex)。任何失败都返回 (None, None)。"""
    try:
        pe = pefile.PE(str(root/'DFO.exe'), fast_load=True)
        try:
            base = pe.OPTIONAL_HEADER.ImageBase
            def literal(va):
                pointer = struct.unpack('<Q', pe.get_data(va-base, 8))[0]
                return pe.get_data(pointer-base, 4096).split(b'\0')[0]
            return literal(0x14dc98110), literal(0x14dc98118)
        finally:
            pe.close()
    except Exception:
        return None, None

def _scan_literals(root):
    """2.38.3.25 起固定 VA 槽位只剩运行期残值（指针 0x127），改为扫描 DFO.exe 文件体：
    每个 PEM 块配对其结束后 0x200 窗口内首个 64 位大写十六进制串。真伪由
    _derive_keys 用 sk.dat 首块试解筛选（exe 里还有一把无关的 MIICeQ 钥）。"""
    exe = (root/'DFO.exe').read_bytes()
    for match in _PEM_BLOCK.finditer(exe):
        found = _AES_HEX.search(exe[match.end():match.end()+0x200])
        if found is not None:
            yield match.group(0), found.group(0)

def _derive_keys(pem, aes_hex, raw):
    """用候选 (PEM, AES hex) 解 sk.dat；任一步失败返回 None 而非抛错（候选筛选）。"""
    try:
        private = load_pem_private_key(pem, password=None)
        size = private.key_size//8
        if not raw or len(raw)%size:
            return None
        private.decrypt(raw[:size], PKCS1v15())
        data = b''.join(private.decrypt(raw[i:i+size], PKCS1v15()) for i in range(0, len(raw), size))
        prefix = len(data)//256*256
        data = aes(bytes.fromhex(aes_hex.decode()), data[:prefix])+data[prefix:]
        if len(data)%32:
            return None
        return [data[i:i+32] for i in range(0, len(data), 32)]
    except Exception:
        return None

def wrapper_keys(root):
    root = Path(root)
    raw = (root/'sk.dat').read_bytes()
    fixed = _fixed_va_literals(root)
    if fixed[0] is not None and fixed[1] is not None:
        keys = _derive_keys(fixed[0], fixed[1], raw)
        if keys is not None:
            return keys
    for pem, aes_hex in _scan_literals(root):
        keys = _derive_keys(pem, aes_hex, raw)
        if keys is not None:
            return keys
    raise RuntimeError('DFO.exe 中找不到能解密 sk.dat 的 wrapper 密钥（固定 VA 与全文件扫描候选均试解失败）')

class Archive:
    def __init__(self, source, client_root=None):
        self.source = Path(source)
        self.data = bytearray(self.source.read_bytes())
        self.source_sha256 = hashlib.sha256(self.data).hexdigest()
        self.keys = wrapper_keys(Path(client_root) if client_root else self.source.parent)
        for index, key in enumerate(self.keys):
            at = index * 0xa00000
            if at >= len(self.data):
                break
            self.data[at:at+0x2800] = aes(key, bytes(self.data[at:at+0x2800]))
        self.header = stream('iNfO', self.data[:48])
        assert self.header[:4] == b'nkpi'
        self.count = struct.unpack_from('<I', self.header, 24)[0]
        self.body_size, self.group_count, hash_size, name_size = struct.unpack_from('<4I', self.header, 32)
        name_offset = 48 + self.count * 24 + hash_size
        position = name_offset + 8
        self.pools = []
        for key, magic in (('stAs', 0xe7adf7ea), ('stWs', 0xb8dea7ac)):
            size, original = struct.unpack_from('<II', self.data, position)
            size ^= magic
            position += 8
            raw = self.data[position:position+size]
            seed = (0x339e9711*ord(key[0])+0x393*(ord(key[3])+0x393*(ord(key[2])+0x393*ord(key[1])))) & 0xffffffff
            for offset in range(0, len(raw), 4):
                first = (0x343fd*seed+0x269ec9) & 0xffffffff
                seed = (0x343fd*first+0x269ec9) & 0xffffffff
                mask = struct.pack('<I', (first & 0xffff0000)+(seed >> 16))
                for i in range(min(4, len(raw)-offset)):
                    raw[offset+i] ^= mask[i]
            pool = zlib.decompress(raw)
            assert len(pool) == (original ^ size)
            self.pools.append(pool)
            position += size
        self.group_offset = name_offset + name_size
        self.body_offset = self.group_offset + self.group_count * 8
        assert self.body_offset + self.body_size == len(self.data)
        self.groups = stream('Gidx', self.data[self.group_offset:self.body_offset])
        self.files = {}
        for index in range(self.count):
            name, path, group, offset, size, kind = self.record(index)
            if kind not in (1, 3):
                continue
            filename = self.resolve(name).lower()
            if filename.endswith(('.stk', '.equ', '.str', '.lst')):
                full = (self.resolve(path)+'/'+filename).lower().lstrip('/')
                self.files[full] = index

    def record(self, index):
        return struct.unpack_from('<6i', self.data, 48+index*24)

    @lru_cache(maxsize=150000)
    def resolve(self, offset):
        if offset & 1:
            start = offset & ~1
            end = start
            while self.pools[1][end:end+2] != b'\0\0':
                end += 2
            return self.pools[1][start:end].decode('utf-16-le')
        start = offset >> 1
        return self.pools[0][start:self.pools[0].find(b'\0', start)].decode('utf-8')

    @lru_cache(maxsize=16)
    def chunk(self, group):
        end, size = struct.unpack_from('<2I', self.groups, group*8)
        start = struct.unpack_from('<I', self.groups, (group-1)*8)[0] if group else 0
        raw = zlib.decompress(stream('mAIn', self.data[self.body_offset+start:self.body_offset+end]))
        assert len(raw) == size
        return raw

    def read(self, path):
        _, _, group, offset, size, kind = self.record(self.files[path.lower()])
        return self.chunk(group)[offset:offset+size]

    def tokens(self, path):
        return [(typ, self.resolve(value) if typ in (3, 6, 8) else value)
                for typ, value in struct.iter_unpack('<Bi', self.read(path))]

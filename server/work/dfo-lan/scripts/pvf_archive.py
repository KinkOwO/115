"""只读解析当前客户端 PVF，供规则导出使用；不提供资源修改操作。"""
from functools import lru_cache
from pathlib import Path
import struct, hashlib, json, zlib
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

def wrapper_keys(root):
    pe = pefile.PE(str(root/'DFO.exe'), fast_load=True)
    base = pe.OPTIONAL_HEADER.ImageBase
    def literal(va):
        pointer = struct.unpack('<Q', pe.get_data(va-base, 8))[0]
        return pe.get_data(pointer-base, 4096).split(b'\0')[0]
    private = load_pem_private_key(literal(0x14dc98110), password=None)
    raw = (root/'sk.dat').read_bytes()
    size = private.key_size//8
    assert len(raw)%size == 0
    raw = b''.join(private.decrypt(raw[i:i+size], PKCS1v15()) for i in range(0, len(raw), size))
    prefix = len(raw)//256*256
    raw = aes(bytes.fromhex(literal(0x14dc98118).decode()), raw[:prefix])+raw[prefix:]
    assert len(raw)%32 == 0
    pe.close()
    return [raw[i:i+32] for i in range(0, len(raw), 32)]

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

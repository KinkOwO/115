"""wrapper_keys 扫描回退测试：合成 RSA/AES fixture，不含任何真实客户端密钥。

背景：2.38.3.25 的 DFO.exe 里固定 VA 槽位（0x14DC98110/0x14DC98118）只剩
运行期残值，wrapper_keys 需要回退到"全文件扫描 PEM 块 + PEM 后 0x200 窗口
内找 64 位大写十六进制 AES 钥"，并用 sk.dat 首块 RSA 试解筛选真钥
（exe 里还存在一把无关的 MIICeQ 钥，必须被剔除）。
"""
import os
import tempfile
import unittest
from pathlib import Path

from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import padding, rsa

import pvf_archive
import reinforcement_pvf_reader

MODULES = (pvf_archive, reinforcement_pvf_reader)


def export_pem(key):
    return key.private_bytes(
        serialization.Encoding.PEM,
        serialization.PrivateFormat.PKCS8,
        serialization.NoEncryption(),
    )


def build_client(root, *, with_decoy=True):
    """合成客户端：DFO.exe 内嵌（诱饵+真实）PEM 与 AES hex，sk.dat 由真实钥加密。

    返回 (client_dir, expected_keys)。真实钥派生路径与官方一致：
    plain -> AES-CBC -> RSA(PKCS1v15) 分块 -> sk.dat。
    """
    key = rsa.generate_private_key(public_exponent=65537, key_size=2048)
    aes_key = os.urandom(32)
    plain = os.urandom(480)  # PKCS1v15 单块上限 245：按 240 分块 -> 2 块；prefix=256 只加密前 256 字节
    aes_ct = pvf_archive.aes(aes_key, plain, encrypt=True)
    public = key.public_key()
    sk = b''.join(
        public.encrypt(aes_ct[i:i + 240], padding.PKCS1v15())
        for i in range(0, len(aes_ct), 240)
    )
    blob = bytearray(b'fake-exe-pad\x00' * 16)
    if with_decoy:
        decoy = rsa.generate_private_key(public_exponent=65537, key_size=2048)
        blob += export_pem(decoy) + b'\x00' * 64
    blob += export_pem(key) + b'\x00' * 16 + aes_key.hex().upper().encode() + b'\x00' + b'tail'
    client = root / 'client'
    client.mkdir(parents=True)
    (client / 'DFO.exe').write_bytes(bytes(blob))
    (client / 'sk.dat').write_bytes(sk)
    # prefix = 480//256*256 = 256：只有前 256 字节被 AES 解密，其余原样透传。
    mixed = pvf_archive.aes(aes_key, aes_ct[:256]) + aes_ct[256:]
    expected = [mixed[i:i + 32] for i in range(0, len(mixed), 32)]
    return client, expected


class WrapperKeysScanTests(unittest.TestCase):
    def test_scan_fallback_recovers_keys(self):
        """非 PE 的 DFO.exe（固定 VA 必失败）+ 诱饵钥在前，扫描回退仍取回正确钥。"""
        for module in MODULES:
            with self.subTest(module=module.__name__):
                with tempfile.TemporaryDirectory() as tmp:
                    client, expected = build_client(Path(tmp))
                    self.assertEqual(module.wrapper_keys(client), expected)

    def test_no_usable_key_raises(self):
        """只有诱饵钥时必须显式报错，而不是返回错误密钥。"""
        for module in MODULES:
            with self.subTest(module=module.__name__):
                with tempfile.TemporaryDirectory() as tmp:
                    client, _ = build_client(Path(tmp))
                    # 抹掉真实钥：重写 sk.dat 为对诱钥才有意义的随机块也会被
                    # 试解筛掉；这里直接换成无法解密的随机数据更直接。
                    (client / 'sk.dat').write_bytes(os.urandom(512))
                    with self.assertRaises(RuntimeError):
                        module.wrapper_keys(client)


if __name__ == '__main__':
    unittest.main()

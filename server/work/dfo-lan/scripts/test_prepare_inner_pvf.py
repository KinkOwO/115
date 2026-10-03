import tempfile
import unittest
import hashlib
from pathlib import Path
from unittest.mock import patch

from prepare_inner_pvf import prepare
from pvf_archive import aes, stream


class PrepareInnerPVFTest(unittest.TestCase):
    def test_refuses_client_output_and_existing_output_before_reading_keys(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            client = root / "client"
            client.mkdir()
            for output in (client / "copy.pvf", client):
                with self.assertRaises(ValueError):
                    prepare(client, output, root / "manifest.json")
            existing = root / "existing.pvf"
            existing.write_bytes(b"preserved")
            with self.assertRaises(FileExistsError):
                prepare(client, existing, root / "manifest.json")
            self.assertEqual(existing.read_bytes(), b"preserved")
            with self.assertRaises(ValueError):
                prepare(client, root / "same", root / "same")

    def test_prepares_a_separate_copy_and_manifest_without_mutating_inputs(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            client = root / "client"
            client.mkdir()
            (client / "DFO.exe").write_bytes(b"exe fixture")
            (client / "sk.dat").write_bytes(b"key file fixture")
            key = bytes(32)
            inner = stream("iNfO", b"nkpi" + bytes(44)) + bytes(0x2800 - 48)
            outer = aes(key, inner, encrypt=True)
            (client / "Script.pvf").write_bytes(outer)
            with patch("prepare_inner_pvf.wrapper_keys", return_value=[key]):
                manifest = prepare(client, root / "inner.pvf", root / "manifest.json")
            self.assertEqual((root / "inner.pvf").read_bytes(), inner)
            self.assertEqual((client / "Script.pvf").read_bytes(), outer)
            self.assertEqual(manifest["inner"]["sha256"], hashlib.sha256(inner).hexdigest())
            self.assertEqual(manifest["outer"]["sha256"], hashlib.sha256(outer).hexdigest())
            self.assertEqual(list(root.glob("*.partial")), [])


if __name__ == "__main__":
    unittest.main()

"""Four-state gate tests for ensure_inner_pvf (no real PVF, no real prepare)."""
import json
import tempfile
import time
import unittest
from pathlib import Path
from unittest.mock import patch

import ensure_inner_pvf as gate


def make_client(root, exe=b"exe", sk=b"sk", script=b"script"):
    client = root / "client"
    client.mkdir(parents=True, exist_ok=True)
    for name, data in (("DFO.exe", exe), ("sk.dat", sk), ("Script.pvf", script)):
        (client / name).write_bytes(data)
    return client


def write_inner(root, data=b"inner-bytes"):
    inner = root / "Script.inner.pvf"
    inner.write_bytes(data)
    return inner


def make_manifest(root, client, inner):
    """A manifest that matches the given client triple (as prepare would write)."""
    mf = {
        "format": gate.FORMAT,
        "decoder": "pvf_archive.wrapper_keys/aes",
        "client_exe": gate.fingerprint(client / "DFO.exe"),
        "sk_dat": gate.fingerprint(client / "sk.dat"),
        "outer": gate.fingerprint(client / "Script.pvf"),
        "inner": gate.fingerprint(inner),
        "cache": gate.client_states(client),
    }
    return mf


def write_manifest(root, mf):
    path = root / "Script.inner.manifest.json"
    path.write_text(json.dumps(mf, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    return path


class DecideTests(unittest.TestCase):
    def test_state_table(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            client = make_client(root)
            inner = write_inner(root)
            good = make_manifest(root, client, inner)

            # no inner -> build
            gone = root / "gone.pvf"
            self.assertEqual(gate.decide(client, gone, good)[0], True)

            # no manifest -> rebuild (untrusted)
            self.assertEqual(gate.decide(client, inner, None)[0], True)
            self.assertIn("不可信", gate.decide(client, inner, None)[1])

            # matching -> reuse
            needs, reason = gate.decide(client, inner, good)
            self.assertFalse(needs, reason)

    def test_bad_format_is_untrusted(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            client = make_client(root)
            inner = write_inner(root)
            mf = make_manifest(root, client, inner)
            mf["format"] = "something-else"
            self.assertTrue(gate.decide(client, inner, mf)[0])

    def test_detects_change_in_exe_only(self):
        """Only DFO.exe changes -> must rebuild (key material lives in it)."""
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            client = make_client(root)
            inner = write_inner(root)
            mf = make_manifest(root, client, inner)
            self.assertFalse(gate.decide(client, inner, mf)[0])

            time.sleep(0.01)
            (client / "DFO.exe").write_bytes(b"exe-changed")
            needs, reason = gate.decide(client, inner, mf)
            self.assertTrue(needs, reason)
            self.assertIn("已变化", reason)

    def test_detects_change_in_sk_dat_only(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            client = make_client(root)
            inner = write_inner(root)
            mf = make_manifest(root, client, inner)

            time.sleep(0.01)
            (client / "sk.dat").write_bytes(b"sk-changed")
            self.assertTrue(gate.decide(client, inner, mf)[0])

    def test_truncated_inner_is_rebuilt_even_if_client_unchanged(self):
        """The inner file itself must match the manifest, independent of the client."""
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            client = make_client(root)
            inner = write_inner(root, b"inner-original")
            mf = make_manifest(root, client, inner)

            inner.write_bytes(b"xx")  # truncate
            needs, reason = gate.decide(client, inner, mf)
            self.assertTrue(needs, reason)
            self.assertIn("大小不符", reason)

    def test_fast_path_skips_hashing_when_states_match(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            client = make_client(root)
            inner = write_inner(root)
            mf = make_manifest(root, client, inner)
            with patch.object(gate, "fingerprint", side_effect=AssertionError("不应做全量哈希")):
                needs, reason = gate.decide(client, inner, mf)
            self.assertFalse(needs, reason)


class RotateTests(unittest.TestCase):
    def test_rotates_instead_of_deleting(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            inner = write_inner(root, b"old-inner")
            mf = write_manifest(root, {"format": gate.FORMAT})

            rotated = gate.rotate_stale([inner, mf])
            self.assertIsNotNone(rotated)
            self.assertFalse(inner.exists())
            self.assertFalse(mf.exists())
            backups = list(root.glob("Script.inner.pvf.stale-*"))
            self.assertEqual(len(backups), 1)
            self.assertEqual(backups[0].read_bytes(), b"old-inner")

    def test_noop_when_absent(self):
        with tempfile.TemporaryDirectory() as tmp:
            self.assertIsNone(gate.rotate_stale([Path(tmp) / "nope.pvf"]))


class EnsureTests(unittest.TestCase):
    def test_rejects_incomplete_client(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            client = make_client(root)
            (client / "sk.dat").unlink()
            with self.assertRaises(RuntimeError) as ctx:
                gate.ensure(client, root / "in.pvf", root / "m.json", lambda *a: {})
            self.assertIn("sk.dat", str(ctx.exception))

    def test_reuse_skips_prepare(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            client = make_client(root)
            inner = write_inner(root)
            mf = write_manifest(root, make_manifest(root, client, inner))

            def boom(*args):
                raise AssertionError("不应触发生成")

            result = gate.ensure(client, inner, mf, boom, log=lambda *a: None)
            self.assertFalse(result["generated"])

    def test_builds_when_missing_and_writes_cache(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            client = make_client(root)
            inner = root / "Script.inner.pvf"
            mf_path = root / "Script.inner.manifest.json"

            def prepare(client_dir, output, manifest_path):
                Path(output).write_bytes(b"fresh-inner")
                produced = make_manifest(root, client, Path(output))
                Path(manifest_path).write_text(json.dumps(produced), encoding="utf-8")
                return produced

            result = gate.ensure(client, inner, mf_path, prepare, log=lambda *a: None)
            self.assertTrue(result["generated"])
            self.assertTrue(inner.is_file())

            # cache backfilled -> a second call must reuse.
            again = gate.ensure(client, inner, mf_path, prepare, log=lambda *a: None)
            self.assertFalse(again["generated"])

    def test_rotates_stale_before_rebuild(self):
        """An existing inner must be moved aside: prepare refuses an existing output."""
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            client = make_client(root)
            inner = write_inner(root, b"stale-inner")
            # manifest from a DIFFERENT client -> stale
            other = make_client(root / "other", exe=b"other-exe")
            mf_path = write_manifest(root, make_manifest(root, other, inner))

            seen = {}

            def prepare(client_dir, output, manifest_path):
                seen["inner_existed"] = Path(output).exists()
                Path(output).write_bytes(b"fresh")
                produced = make_manifest(root, client, Path(output))
                Path(manifest_path).write_text(json.dumps(produced), encoding="utf-8")
                return produced

            result = gate.ensure(client, inner, mf_path, prepare, log=lambda *a: None)
            self.assertTrue(result["generated"])
            self.assertFalse(seen["inner_existed"], "prepare 前应先轮换掉旧件")
            self.assertTrue(list(root.glob("Script.inner.pvf.stale-*")))

    def test_detects_hash_mismatch_from_prepare(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            client = make_client(root)
            inner = root / "Script.inner.pvf"

            def prepare(client_dir, output, manifest_path):
                Path(output).write_bytes(b"actual")
                return {"inner": {"sha256": "0" * 64}}

            with self.assertRaises(RuntimeError) as ctx:
                gate.ensure(client, inner, root / "m.json", prepare, log=lambda *a: None)
            self.assertIn("哈希不符", str(ctx.exception))


if __name__ == "__main__":
    unittest.main()

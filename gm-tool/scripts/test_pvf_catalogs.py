"""Read-only source routing tests; no HTTP listener, database or child process."""
import importlib.util
import io
import json
import pathlib
import sys
import tempfile
import unittest
import urllib.error
from unittest import mock


def load_module(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


ROOT = pathlib.Path(__file__).resolve().parents[1]


class NativeProxyTests(unittest.TestCase):
    def setUp(self):
        self.proxy = load_module("test_proxy", ROOT / "dashboard/gm_dashboard_proxy.py")

    def test_backend_metadata_needs_no_exported_files(self):
        data = {"source": "test", "items": {"7": ["stackable", "[material]"],
                                             "9": ["equipment", "[coat]"]}, "stackables": [7]}
        with mock.patch.object(self.proxy.urllib.request, "urlopen", return_value=io.BytesIO(json.dumps(data).encode())), \
                mock.patch("builtins.open", side_effect=AssertionError("exported JSON accessed")):
            self.proxy.load_stackable_ids()
            self.proxy.load_category_maps()
            self.assertEqual(self.proxy.categorize("7"), ("物品栏", "材料"))
            self.assertEqual(self.proxy.STACKABLE_IDS, {"7"})
            self.assertEqual(self.proxy.EQ_MAP, {"9": "[coat]"})

    def test_backend_failures_never_fall_back_to_exports(self):
        for code in (401, 404, 500):
            error = urllib.error.HTTPError("local", code, "failure", None, None)
            with mock.patch.object(self.proxy.urllib.request, "urlopen", side_effect=error), \
                    mock.patch("builtins.open", side_effect=AssertionError("game export read")):
                with self.assertRaises(urllib.error.HTTPError):
                    self.proxy.load_catalog_metadata()
            self.assertIsNone(self.proxy.CATALOG_METADATA_LOADED)

    def test_bad_response_does_not_replace_maps(self):
        data = {"items": {"7": ["equipment"]}, "stackables": [7]}
        self.proxy.INDEX_MAP = {"old": ("stackable", "[waste]")}
        with mock.patch.object(self.proxy.urllib.request, "urlopen", return_value=io.BytesIO(json.dumps(data).encode())):
            with self.assertRaises(ValueError):
                self.proxy.load_catalog_metadata()
        self.assertIn("old", self.proxy.INDEX_MAP)
        self.assertIsNone(self.proxy.CATALOG_METADATA_LOADED)


class NativeLauncherTests(unittest.TestCase):
    def setUp(self):
        self.launcher = load_module("test_launcher", ROOT / "scripts/gmweb.py")

    def test_native_args_exclude_exported_game_catalogs(self):
        args = type("Args", (), {"catalog_source": "pvf", "pvf_archive": "inner.pvf",
                                 "pvf_source_checksum": "1" * 64, "pvf_drop_policy": "policy.json"})()
        native = self.launcher.catalog_args(args)
        args.pvf_source_checksum = ""
        self.assertIn("-pvf-source-checksum", self.launcher.catalog_args(args))
        for flag in ("-loot-catalog", "-equipment-catalog", "-item-index", "-equipment-slots"):
            self.assertNotIn(flag, native)
        args.pvf_archive = ""
        with self.assertRaises(ValueError):
            self.launcher.catalog_args(args)

    def test_check_exits_before_storage_or_network(self):
        with tempfile.TemporaryDirectory() as directory:
            binary = pathlib.Path(directory) / "candidate.exe"
            binary.write_bytes(b"candidate placeholder")
            argv = ["gmweb.py", "--check", "--catalog-source", "pvf", "--gmweb-binary", str(binary),
                    "--pvf-archive", "inner.pvf", "--pvf-source-checksum", "1" * 64,
                    "--storage", "missing/storage.json"]
            with mock.patch.object(sys, "argv", argv), \
                    mock.patch.object(self.launcher.subprocess, "run") as run, \
                    mock.patch.object(self.launcher, "load_storage", side_effect=AssertionError("storage read")), \
                    mock.patch.object(self.launcher, "start_storage", side_effect=AssertionError("storage start")), \
                    mock.patch.object(self.launcher, "listening", side_effect=AssertionError("network check")):
                run.return_value = type("Result", (), {"stdout": "", "stderr": "", "returncode": 0})()
                self.launcher.main()
                run.assert_called_once()
                self.assertIn("-check-catalogs", run.call_args.args[0])
                self.assertTrue(run.call_args.kwargs["capture_output"])

    def test_default_native_paths_follow_storage_module(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            binary = root / "candidate.exe"
            binary.write_bytes(b"candidate placeholder")
            module = root / "game/server/work/dfo-lan"
            storage = module / "runtime/storage/missing.json"
            argv = ["gmweb.py", "--check", "--gmweb-binary", str(binary), "--storage", str(storage)]
            with mock.patch.object(sys, "argv", argv), \
                    mock.patch.object(self.launcher.subprocess, "run") as run, \
                    mock.patch.object(self.launcher, "load_storage", side_effect=AssertionError("storage read")), \
                    mock.patch.object(self.launcher, "start_storage", side_effect=AssertionError("storage start")):
                run.return_value = type("Result", (), {"stdout": "", "stderr": "", "returncode": 0})()
                self.launcher.main()
                command = run.call_args.args[0]
                self.assertEqual(command[command.index("-catalog-source") + 1], "pvf")
                self.assertEqual(command[command.index("-pvf-archive") + 1], str(module.parent / "client-build/Script.inner.pvf"))
                self.assertEqual(command[command.index("-pvf-drop-policy") + 1], str(module / "configs/pvf-drop-policy.json"))
                self.assertNotIn("-item-index", command)


if __name__ == "__main__":
    unittest.main()

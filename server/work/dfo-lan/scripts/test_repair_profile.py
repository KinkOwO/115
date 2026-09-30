import json
import pathlib
import tempfile
import unittest
from repair_profile import load_profile


class RepairProfileTests(unittest.TestCase):
    def test_example_is_portable_and_policies_are_opt_in(self):
        project = pathlib.Path(__file__).resolve().parent.parent
        binary, required, env = load_profile(project / 'configs/repair-profile.example.json', project)
        self.assertEqual(binary, project / 'bin/wireprobe-repairs.exe')
        self.assertEqual(env['DFO_ODYSSEY_TEMPORARY_CREDITS'], '0')
        self.assertNotIn('DFO_ODYSSEY_COIN_RULES', env)
        self.assertNotIn('DFO_FATIGUE_RULES', env)
        self.assertTrue(all(p.is_absolute() for p in required))
        self.assertNotIn('DFO_CHARACTER_STORAGE', env)

    def test_rejects_unknown_keys(self):
        with tempfile.TemporaryDirectory() as directory:
            p = pathlib.Path(directory) / 'profile.json'
            p.write_text(json.dumps({'binary':'server.exe', 'environment': {'DFO_CHARACTER_STORAGE':'other-db.json'}}))
            with self.assertRaises(ValueError):
                load_profile(p, pathlib.Path(directory))

    def test_pvf_candidate_is_explicit_and_source_bound(self):
        project = pathlib.Path(__file__).resolve().parent.parent
        binary, required, env = load_profile(project / 'configs/pvf-direct-candidate.json', project)
        self.assertEqual(binary, project / '.tmp/bin/wireprobe-handoff-source.exe')
        self.assertIn('DFO_PVF_ARCHIVE', env)
        self.assertEqual(len(env['DFO_PVF_SHA256']), 64)
        self.assertEqual(set(env), {'DFO_PVF_ARCHIVE', 'DFO_PVF_SHA256', 'DFO_PVF_CATALOGS'})
        self.assertIn(pathlib.Path(env['DFO_PVF_ARCHIVE']), required)

    def test_invalid_pvf_domains_and_checksums_rejected(self):
        for key, value in [('DFO_PVF_CATALOGS', 'items,items'),
                           ('DFO_PVF_CATALOGS', 'characters'),
                           ('DFO_PVF_CATALOGS', 'items,'),
                           ('DFO_PVF_CATALOGS', ''),
                           ('DFO_PVF_SHA256', 'not-a-checksum')]:
            with self.subTest(key=key, value=value), tempfile.TemporaryDirectory() as directory:
                p = pathlib.Path(directory) / 'profile.json'
                p.write_text(json.dumps({'binary': 'server.exe', 'environment': {key: value}}))
                with self.assertRaises(ValueError):
                    load_profile(p, pathlib.Path(directory))


if __name__ == '__main__':
    unittest.main()

import json
import pathlib
import tempfile
import unittest
from repair_profile import load_profile


class RestoredProfileTests(unittest.TestCase):
    def test_paired_release_profile_keeps_all_r2_domains_and_gm(self):
        module = pathlib.Path(__file__).resolve().parents[1]
        _, _, env = load_profile(module / 'configs/pvf-default.json', module)
        domains = env['DFO_PVF_CATALOGS'].split(',')
        self.assertEqual(len(domains), 62)
        self.assertIn('transform', domains)
        self.assertTrue({'attendance-login', 'attendance-weekly', 'attendance-online', 'attendance-daily', 'membership'}.issubset(domains))
        self.assertEqual(env['DFO_LOCAL_GM'], '1')
        self.assertEqual(env['DFO_ELITE_COMPANION_PILOT'], 'all')

    def test_duplicate_or_unknown_activity_domain_still_rejected(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = pathlib.Path(tmp)
            for selection in ('membership,membership', 'attendance-online-unknown'):
                p = root / 'profile.json'
                p.write_text(json.dumps({'binary': 'server.exe', 'environment': {'DFO_PVF_CATALOGS': selection}}), 'utf-8')
                with self.assertRaises(ValueError):
                    load_profile(p, root)

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


if __name__ == '__main__':
    unittest.main()

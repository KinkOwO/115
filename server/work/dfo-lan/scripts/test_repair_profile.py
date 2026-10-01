import json
import pathlib
import tempfile
import unittest
from repair_profile import load_profile


class RepairProfileTests(unittest.TestCase):
    def test_native_character_profile_preserves_save_source_and_policy_only_paths(self):
        project = pathlib.Path(__file__).resolve().parent.parent
        binary, required, env = load_profile(project / 'configs/pvf-characters-candidate.json', project)
        self.assertEqual(len(env['DFO_PVF_CATALOGS'].split(',')), 51)
        self.assertEqual(binary, project / '.tmp/pvf-characters/bin/wireprobe-handoff-source.exe')
        policy = json.loads(pathlib.Path(env['DFO_PVF_CHARACTER_POLICY']).read_text(encoding='utf-8'))
        self.assertEqual(policy['source_checksum'], env['DFO_PVF_SHA256'])
        self.assertEqual(len(policy['initial_skill_slots']), 17)
        self.assertFalse(policy['enable_source_commands'])
        self.assertFalse(policy['enable_advancement_shortcuts'])
        self.assertTrue(all(p.suffix != '.json' or 'policy' in p.name for p in required))
        self.assertNotIn('DFO_CHARACTER_CATALOG', env)

    def test_example_is_portable_and_policies_are_opt_in(self):
        project = pathlib.Path(__file__).resolve().parent.parent
        binary, required, env = load_profile(project / 'configs/repair-profile.example.json', project)
        self.assertEqual(binary, project / 'bin/wireprobe-repairs.exe')
        self.assertEqual(env['DFO_ODYSSEY_TEMPORARY_CREDITS'], '0')
        self.assertNotIn('DFO_ODYSSEY_COIN_RULES', env)
        self.assertNotIn('DFO_FATIGUE_RULES', env)
        self.assertTrue(all(p.is_absolute() for p in required))
        self.assertNotIn('DFO_CHARACTER_STORAGE', env)

    def test_odyssey_candidate_keeps_prior_content_policy_compatible(self):
        project = pathlib.Path(__file__).resolve().parent.parent
        binary, required, env = load_profile(project / 'configs/pvf-odyssey-candidate.json', project)
        self.assertEqual(len(env['DFO_PVF_CATALOGS'].split(',')), 35)
        self.assertEqual(binary, project / '.tmp/pvf-odyssey/bin/wireprobe-handoff-source.exe')
        self.assertEqual(pathlib.Path(env['DFO_PVF_CONTENT_POLICY']), project / 'configs/pvf-odyssey-policy.json')
        old_policy = json.loads((project / 'configs/pvf-content-policy.json').read_text(encoding='utf-8'))
        self.assertEqual(set(old_policy), {'version', 'attunement_dungeons'})
        old_binary, _, old_env = load_profile(project / 'configs/pvf-content-candidate.json', project)
        self.assertEqual(len(old_env['DFO_PVF_CATALOGS'].split(',')), 30)
        self.assertNotEqual(old_binary, binary)

    def test_rejects_unknown_keys(self):
        with tempfile.TemporaryDirectory() as directory:
            p = pathlib.Path(directory) / 'profile.json'
            p.write_text(json.dumps({'binary':'server.exe', 'environment': {'DFO_CHARACTER_STORAGE':'other-db.json'}}))
            with self.assertRaises(ValueError):
                load_profile(p, pathlib.Path(directory))

    def test_cashshop_native_profile_retains_release_policy_without_export(self):
        project = pathlib.Path(__file__).resolve().parent.parent
        binary, required, env = load_profile(project / 'configs/pvf-cashshop-candidate.json', project)
        self.assertEqual(len(env['DFO_PVF_CATALOGS'].split(',')), 52)
        self.assertEqual(binary, project / '.tmp/pvf-cashshop/bin/wireprobe-handoff-source.exe')
        self.assertEqual(env['DFO_SHOP_RELEASE'], '1')
        self.assertNotIn('DFO_SHOP_PURCHASE_PILOT', env)
        self.assertTrue(all(p.suffix != '.json' or 'policy' in p.name for p in required))

    def test_box_native_profile_keeps_only_scope_and_placement_policy(self):
        project = pathlib.Path(__file__).resolve().parent.parent
        binary, required, env = load_profile(project / 'configs/pvf-boxes-candidate.json', project)
        self.assertEqual(len(env['DFO_PVF_CATALOGS'].split(',')), 53)
        self.assertEqual(binary, project / '.tmp/pvf-boxes/bin/wireprobe-handoff-source.exe')
        policy = json.loads(pathlib.Path(env['DFO_PVF_BOX_POLICY']).read_text(encoding='utf-8'))
        self.assertEqual(policy['templates'], [590712474, 590719043])
        self.assertNotIn('rewards', policy)
        self.assertNotIn('tables', policy)
        self.assertTrue(all(p.suffix != '.json' or 'policy' in p.name for p in required))

    def test_item_shop_profile_preserves_route_policy_without_offers(self):
        project = pathlib.Path(__file__).resolve().parent.parent
        binary, required, env = load_profile(project / 'configs/pvf-item-shops-candidate.json', project)
        self.assertEqual(len(env['DFO_PVF_CATALOGS'].split(',')), 54)
        self.assertEqual(binary, project / '.tmp/pvf-item-shops/bin/wireprobe-handoff-source.exe')
        policy = json.loads(pathlib.Path(env['DFO_PVF_ITEM_SHOP_POLICY']).read_text(encoding='utf-8'))
        self.assertEqual(len(policy['routes']), 527)
        self.assertEqual(policy['purchase_limit_mode'], 'disabled')
        self.assertTrue(all(set(r) <= {'server_shop_id', 'native_shop_id', 'script_path'} for r in policy['routes']))
        self.assertTrue(all(p.suffix != '.json' or 'policy' in p.name for p in required))

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
                           ('DFO_PVF_CATALOGS', 'characterz'),
                           ('DFO_PVF_CATALOGS', 'items,'),
                           ('DFO_PVF_CATALOGS', ''),
                           ('DFO_PVF_SHA256', 'not-a-checksum')]:
            with self.subTest(key=key, value=value), tempfile.TemporaryDirectory() as directory:
                p = pathlib.Path(directory) / 'profile.json'
                p.write_text(json.dumps({'binary': 'server.exe', 'environment': {key: value}}))
                with self.assertRaises(ValueError):
                    load_profile(p, pathlib.Path(directory))

    def test_next_pvf_profile_is_isolated_and_does_not_require_exported_data(self):
        project = pathlib.Path(__file__).resolve().parent.parent
        binary, required, env = load_profile(project / 'configs/pvf-next-candidate.json', project)
        self.assertEqual(binary, project / '.tmp/pvf-next/bin/wireprobe-handoff-source.exe')
        self.assertEqual(env['DFO_PVF_VERIFY_BASELINES'], '0')
        self.assertEqual(len(env['DFO_PVF_CATALOGS'].split(',')), 14)
        self.assertEqual(set(required), {binary, pathlib.Path(env['DFO_PVF_ARCHIVE'])})

    def test_enhancement_pvf_profile_keeps_only_separate_policy(self):
        project = pathlib.Path(__file__).resolve().parent.parent
        binary, required, env = load_profile(project / 'configs/pvf-enhancement-candidate.json', project)
        self.assertEqual(binary, project / '.tmp/pvf-enhancement/bin/wireprobe-handoff-source.exe')
        self.assertEqual(env['DFO_PVF_VERIFY_BASELINES'], '0')
        self.assertEqual(len(env['DFO_PVF_CATALOGS'].split(',')), 15)
        self.assertEqual(env['DFO_PVF_ENHANCEMENT_POLICY'], str(project / 'configs/pvf-enhancement-policy.json'))
        self.assertEqual(set(required), {binary, pathlib.Path(env['DFO_PVF_ARCHIVE']), pathlib.Path(env['DFO_PVF_ENHANCEMENT_POLICY'])})

    def test_migration_profile_retains_independent_policies(self):
        project = pathlib.Path(__file__).resolve().parent.parent
        binary, required, env = load_profile(project / 'configs/pvf-migration-candidate.json', project)
        self.assertEqual(binary, project / '.tmp/pvf-migration/bin/wireprobe-handoff-source.exe')
        self.assertEqual(env['DFO_PVF_VERIFY_BASELINES'], '0')
        self.assertEqual(len(env['DFO_PVF_CATALOGS'].split(',')), 28)
        expected = {binary, pathlib.Path(env['DFO_PVF_ARCHIVE'])}
        for key in ['DFO_PVF_ENHANCEMENT_POLICY', 'DFO_PVF_VAULT_POLICY', 'DFO_PVF_DROP_POLICY', 'DFO_PVF_SCENE_POLICY']:
            expected.add(pathlib.Path(env[key]))
        self.assertEqual(set(required), expected)


if __name__ == '__main__':
    unittest.main()

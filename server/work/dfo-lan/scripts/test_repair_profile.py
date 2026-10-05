import json
import pathlib
import tempfile
import unittest
from repair_profile import load_profile


class RepairProfileTests(unittest.TestCase):
    def test_hell_drop_percent_is_numeric_policy(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            path = root / 'profile.json'
            for value in ('0', '100', '10000', '-1', '10001', '1.5', True):
                with self.subTest(value=value):
                    path.write_text(json.dumps({'binary': 'server.exe', 'environment': {'DFO_HELL_PARTY_DROP_PERCENT': value}}))
                    if value in ('0', '100', '10000'):
                        _, _, env = load_profile(path, root)
                        self.assertEqual(env['DFO_HELL_PARTY_DROP_PERCENT'], value)
                    else:
                        with self.assertRaises(ValueError):
                            load_profile(path, root)

    def load_explicit_profile(self, project, environment, binary='isolated/server.exe'):
        with tempfile.TemporaryDirectory() as directory:
            path = pathlib.Path(directory) / 'profile.json'
            path.write_text(json.dumps({'binary': binary, 'environment': environment}), encoding='utf-8')
            return load_profile(path, project)

    def test_default_profile_keeps_full_scope_without_exports(self):
        project = pathlib.Path(__file__).resolve().parent.parent
        binary, required, env = load_profile(project / 'configs/pvf-default.json', project)
        self.assertEqual(len(env['DFO_PVF_CATALOGS'].split(',')), 56)
        self.assertEqual(env['DFO_PVF_VERIFY_BASELINES'], '0')
        self.assertEqual(binary, project / 'bin/wireprobe-pvf.exe')
        self.assertTrue(all(p.suffix != '.json' or 'policy' in p.name for p in required))

    def test_native_character_profile_preserves_save_source_and_policy_only_paths(self):
        project = pathlib.Path(__file__).resolve().parent.parent
        _, required, env = load_profile(project / 'configs/pvf-default.json', project)
        self.assertIn('characters', env['DFO_PVF_CATALOGS'].split(','))
        policy = json.loads(pathlib.Path(env['DFO_PVF_CHARACTER_POLICY']).read_text(encoding='utf-8'))
        self.assertEqual(policy['source_checksum'], env['DFO_PVF_SHA256'])
        self.assertEqual(len(policy['initial_skill_slots']), 17)
        self.assertFalse(policy['enable_source_commands'])
        self.assertFalse(policy['enable_advancement_shortcuts'])
        self.assertTrue(all(p.suffix != '.json' or 'policy' in p.name for p in required))
        self.assertNotIn('DFO_CHARACTER_CATALOG', env)

    def test_example_is_portable_and_policies_are_opt_in(self):
        project = pathlib.Path(__file__).resolve().parent.parent
        binary, required, env = load_profile(project / 'docs/repair-profile.example.json', project)
        self.assertEqual(binary, project / 'bin/wireprobe-handoff-source.exe')
        self.assertEqual(len(env['DFO_PVF_CATALOGS'].split(',')), 56)
        self.assertNotIn('DFO_SKILL_CATALOG', env)
        self.assertNotIn('DFO_ODYSSEY_DUNGEON_CATALOG', env)
        self.assertEqual(env['DFO_ODYSSEY_TEMPORARY_CREDITS'], '0')
        self.assertNotIn('DFO_ODYSSEY_COIN_RULES', env)
        self.assertNotIn('DFO_FATIGUE_RULES', env)
        self.assertTrue(all(p.is_absolute() for p in required))
        self.assertNotIn('DFO_CHARACTER_STORAGE', env)

    def test_default_content_policy_keeps_odyssey_scope(self):
        project = pathlib.Path(__file__).resolve().parent.parent
        _, _, env = load_profile(project / 'configs/pvf-default.json', project)
        self.assertTrue({'odyssey-growth', 'odyssey-chapters', 'odyssey-weapons', 'odyssey-drop', 'odyssey-currency'} <= set(env['DFO_PVF_CATALOGS'].split(',')))
        # 2026-10-01 收口后已统一使用 pvf-mine-policy.json 作为 content policy 唯一源，attunement 副本范围由源 ctp 自动发现
        self.assertEqual(pathlib.Path(env['DFO_PVF_CONTENT_POLICY']), project / 'configs/pvf-mine-policy.json')
        self.assertNotIn('DFO_PVF_SELECTION_POLICY', env)

    def test_rejects_unknown_keys(self):
        with tempfile.TemporaryDirectory() as directory:
            p = pathlib.Path(directory) / 'profile.json'
            p.write_text(json.dumps({'binary':'server.exe', 'environment': {'DFO_CHARACTER_STORAGE':'other-db.json'}}))
            with self.assertRaises(ValueError):
                load_profile(p, pathlib.Path(directory))

    def test_cashshop_native_profile_retains_release_policy_without_export(self):
        project = pathlib.Path(__file__).resolve().parent.parent
        _, required, env = load_profile(project / 'configs/pvf-default.json', project)
        self.assertIn('cashshop', env['DFO_PVF_CATALOGS'].split(','))
        self.assertEqual(env['DFO_SHOP_RELEASE'], '1')
        self.assertNotIn('DFO_SHOP_PURCHASE_PILOT', env)
        self.assertTrue(all(p.suffix != '.json' or 'policy' in p.name for p in required))

    def test_box_native_profile_keeps_only_scope_and_placement_policy(self):
        project = pathlib.Path(__file__).resolve().parent.parent
        _, required, env = load_profile(project / 'configs/pvf-default.json', project)
        self.assertIn('boxes', env['DFO_PVF_CATALOGS'].split(','))
        policy = json.loads(pathlib.Path(env['DFO_PVF_BOX_POLICY']).read_text(encoding='utf-8'))
        self.assertNotIn('templates', policy)
        self.assertNotIn('cos_paths', policy)
        self.assertNotIn('rewards', policy)
        self.assertNotIn('tables', policy)
        self.assertTrue(all(p.suffix != '.json' or 'policy' in p.name for p in required))

    def test_item_shop_profile_preserves_route_policy_without_offers(self):
        project = pathlib.Path(__file__).resolve().parent.parent
        _, required, env = load_profile(project / 'configs/pvf-default.json', project)
        self.assertIn('item-shops', env['DFO_PVF_CATALOGS'].split(','))
        policy = json.loads(pathlib.Path(env['DFO_PVF_ITEM_SHOP_POLICY']).read_text(encoding='utf-8'))
        self.assertEqual(len(policy['routes']), 527)
        self.assertEqual(policy['purchase_limit_mode'], 'disabled')
        self.assertTrue(all(set(r) <= {'server_shop_id', 'native_shop_id', 'script_path'} for r in policy['routes']))
        self.assertTrue(all(p.suffix != '.json' or 'policy' in p.name for p in required))

    def test_pvf_candidate_is_explicit_and_source_bound(self):
        project = pathlib.Path(__file__).resolve().parent.parent
        binary, required, env = self.load_explicit_profile(project, {
            'DFO_PVF_CATALOGS': 'world,items',
            'DFO_PVF_ARCHIVE': 'isolated/inner.pvf', 'DFO_PVF_SHA256': '',
        })
        self.assertEqual(binary, project / 'isolated/server.exe')
        self.assertIn('DFO_PVF_ARCHIVE', env)
        # 方案 C（next142）：默认不钉版本，锁由内层归档实际哈希派生。
        # 空串是合法值，表示"接受内层实际哈希"；显式 64 位 hex 仍被接受（见下一条测试）。
        self.assertEqual(env['DFO_PVF_SHA256'], '')
        self.assertEqual(set(env), {'DFO_PVF_ARCHIVE', 'DFO_PVF_SHA256', 'DFO_PVF_CATALOGS'})
        self.assertIn(pathlib.Path(env['DFO_PVF_ARCHIVE']), required)

    def test_explicit_pvf_checksum_pin_is_still_honoured(self):
        # 自动派生是"默认值"，不是"唯一值"：发布/审计要钉死某一版时仍然写 64 位 hex。
        project = pathlib.Path(__file__).resolve().parent.parent
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            pinned = 'a' * 64
            p = root / 'profile.json'
            p.write_text(json.dumps({'binary': 'server.exe', 'environment': {
                'DFO_PVF_ARCHIVE': 'inner.pvf', 'DFO_PVF_SHA256': pinned.upper()}}))
            _, _, env = load_profile(p, root)
            self.assertEqual(env['DFO_PVF_SHA256'], pinned)

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

    def test_explicit_profile_is_isolated_and_does_not_require_exported_data(self):
        project = pathlib.Path(__file__).resolve().parent.parent
        binary, required, env = self.load_explicit_profile(project, {
            'DFO_PVF_CATALOGS': 'world,items', 'DFO_PVF_VERIFY_BASELINES': '0',
            'DFO_PVF_ARCHIVE': 'isolated/inner.pvf',
        })
        self.assertEqual(binary, project / 'isolated/server.exe')
        self.assertEqual(env['DFO_PVF_VERIFY_BASELINES'], '0')
        self.assertEqual(env['DFO_PVF_CATALOGS'], 'world,items')
        self.assertEqual(set(required), {binary, pathlib.Path(env['DFO_PVF_ARCHIVE'])})

    def test_enhancement_pvf_profile_keeps_only_separate_policy(self):
        project = pathlib.Path(__file__).resolve().parent.parent
        binary, required, env = self.load_explicit_profile(project, {
            'DFO_PVF_CATALOGS': 'items,enhancements', 'DFO_PVF_VERIFY_BASELINES': '0',
            'DFO_PVF_ARCHIVE': 'isolated/inner.pvf',
            'DFO_PVF_ENHANCEMENT_POLICY': 'configs/pvf-enhancement-policy.json',
        })
        self.assertEqual(binary, project / 'isolated/server.exe')
        self.assertEqual(env['DFO_PVF_VERIFY_BASELINES'], '0')
        self.assertEqual(env['DFO_PVF_CATALOGS'], 'items,enhancements')
        self.assertEqual(env['DFO_PVF_ENHANCEMENT_POLICY'], str(project / 'configs/pvf-enhancement-policy.json'))
        self.assertEqual(set(required), {binary, pathlib.Path(env['DFO_PVF_ARCHIVE']), pathlib.Path(env['DFO_PVF_ENHANCEMENT_POLICY'])})

    def test_explicit_profile_retains_independent_policies(self):
        project = pathlib.Path(__file__).resolve().parent.parent
        policies = {
            'DFO_PVF_ENHANCEMENT_POLICY': 'configs/pvf-enhancement-policy.json',
            'DFO_PVF_VAULT_POLICY': 'configs/pvf-vault-policy.json',
            'DFO_PVF_DROP_POLICY': 'configs/pvf-drop-policy.json',
            'DFO_PVF_SCENE_POLICY': 'configs/pvf-scene-policy.json',
        }
        binary, required, env = self.load_explicit_profile(project, {
            'DFO_PVF_CATALOGS': 'items,enhancements,vault,loot,town,dungeons',
            'DFO_PVF_VERIFY_BASELINES': '0', 'DFO_PVF_ARCHIVE': 'isolated/inner.pvf',
            **policies,
        })
        self.assertEqual(binary, project / 'isolated/server.exe')
        self.assertEqual(env['DFO_PVF_VERIFY_BASELINES'], '0')
        expected = {binary, pathlib.Path(env['DFO_PVF_ARCHIVE'])}
        for key, value in policies.items():
            self.assertEqual(pathlib.Path(env[key]), project / value)
            expected.add(pathlib.Path(env[key]))
        self.assertEqual(set(required), expected)


if __name__ == '__main__':
    unittest.main()

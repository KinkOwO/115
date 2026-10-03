"""Default launch and legacy fallback checks without starting any processes."""
import ast
import contextlib
import io
import json
import os
import pathlib
import sys
import tempfile
import types
import unittest
from unittest import mock

import launch_local as launch

PROBE = launch.PROJECT.parent / 'dfo_probe_tools'
sys.path.insert(0, str(PROBE))
from catalog_startup import validate_json_catalogs


def arguments(**overrides):
    values = dict(repair_profile=None, source_build=False, json_mode=False, pvf_mode=False,
                  client_only=False, storage_only=False, server_only=False, check=False)
    values.update(overrides)
    return types.SimpleNamespace(**values)


class DefaultPVFLaunchTests(unittest.TestCase):
    def test_explicit_pvf_profile_uses_current_default_scope(self):
        binary, required, env = launch.gateway_configuration(arguments(pvf_mode=True), {'server_binary': 'legacy.exe'})
        _, configured, profile = launch.load_profile(launch.DEFAULT_PVF_PROFILE, launch.PROJECT)
        self.assertEqual(binary, launch.PROJECT / 'bin/wireprobe-pvf.exe')
        self.assertEqual(env, profile)
        self.assertEqual(required, configured)
        self.assertEqual(len(env['DFO_PVF_CATALOGS'].split(',')), 54)
        self.assertTrue(all(p.suffix != '.json' or 'policy' in p.name for p in required))
        with mock.patch.dict(os.environ, {'DFO_ODYSSEY_MODE': '0'}, clear=True):
            self.assertEqual(launch.launch_environment(arguments(), env)['DFO_ODYSSEY_MODE'], '0')
        with mock.patch.dict(os.environ, {'DFO_ODYSSEY_MODE': '1'}, clear=True):
            self.assertEqual(launch.launch_environment(arguments(), env)['DFO_ODYSSEY_MODE'], '1')

    def test_missing_default_profile_fails_without_json_fallback(self):
        with mock.patch.object(launch, 'DEFAULT_PVF_PROFILE', pathlib.Path('missing-default-profile.json')):
            with self.assertRaises(FileNotFoundError):
                launch.gateway_configuration(arguments(pvf_mode=True), {'server_binary': 'legacy.exe'})

    def test_explicit_json_mode_uses_configured_binary_without_loading_pvf_profile(self):
        configured_binary = launch.ROOT / 'legacy.exe'
        with mock.patch.object(launch, 'load_profile', side_effect=AssertionError('JSON mode must not load a PVF profile')):
            binary, required, env = launch.gateway_configuration(
                arguments(json_mode=True), {'server_binary': 'legacy.exe'})
        self.assertEqual(binary, configured_binary)
        self.assertEqual(required, [])
        self.assertEqual(env, {})

    def test_explicit_candidate_and_source_build_are_retained(self):
        with tempfile.TemporaryDirectory() as directory:
            explicit = pathlib.Path(directory) / 'profile.json'
            explicit.write_text(json.dumps({'binary': 'isolated/server.exe', 'environment': {
                'DFO_PVF_CATALOGS': 'items,characters', 'DFO_PVF_ARCHIVE': 'isolated/inner.pvf',
            }}), encoding='utf-8')
            binary, required, env = launch.gateway_configuration(arguments(repair_profile=explicit), {'server_binary': 'legacy.exe'})
            self.assertEqual(binary, launch.PROJECT / 'isolated/server.exe')
            self.assertEqual(env['DFO_PVF_CATALOGS'], 'items,characters')
            self.assertEqual(set(required), {binary, launch.PROJECT / 'isolated/inner.pvf'})
            source_binary, _, source_env = launch.gateway_configuration(
                arguments(repair_profile=explicit, source_build=True), {'server_binary': 'legacy.exe'})
            self.assertEqual(source_binary, binary)
            self.assertEqual(source_env, env)
        binary, required, env = launch.gateway_configuration(
            arguments(source_build=True, pvf_mode=True), {'server_binary': 'legacy.exe'})
        self.assertEqual(binary, launch.PROJECT / 'bin/wireprobe-handoff-source.exe')
        self.assertIn(binary, required)
        self.assertNotIn(launch.PROJECT / 'bin/wireprobe-pvf.exe', required)
        self.assertEqual(len(env['DFO_PVF_CATALOGS'].split(',')), 54)

    def test_source_build_json_mode_does_not_inherit_pvf_profile(self):
        binary, required, profile_env = launch.gateway_configuration(
            arguments(source_build=True, json_mode=True), {'server_binary': 'legacy.exe'})
        self.assertEqual(binary, launch.PROJECT / 'bin/wireprobe-handoff-source.exe')
        self.assertEqual(required, [])
        self.assertEqual(profile_env, {})
        with mock.patch.dict(os.environ, {'DFO_PVF_CATALOGS': 'characters', 'DFO_PVF_ARCHIVE': 'stale.pvf'}, clear=True):
            env = launch.launch_environment(arguments(source_build=True, json_mode=True), profile_env)
        self.assertFalse(any(key.startswith('DFO_PVF_') for key in env))

    def test_json_and_remote_storage_modes_do_not_load_default_source(self):
        for args in (arguments(json_mode=True), arguments(client_only=True), arguments(storage_only=True)):
            with mock.patch.object(launch, 'load_profile', side_effect=AssertionError('native source not needed')):
                _, required, env = launch.gateway_configuration(args, {'server_binary': 'legacy.exe'})
            self.assertEqual((required, env), ([], {}))
        with mock.patch.dict(os.environ, {'DFO_PVF_CATALOGS': 'characters', 'DFO_PVF_ARCHIVE': 'old', 'DFO_ODYSSEY_MODE': '1'}, clear=True):
            env = launch.launch_environment(arguments(json_mode=True), {})
        self.assertFalse(any(k.startswith('DFO_PVF_') for k in env))
        self.assertEqual(env['DFO_ODYSSEY_MODE'], '1')

    def test_native_domain_checks_do_not_open_character_or_dungeon_json(self):
        command = ['server', '-character-catalog', 'missing.json', '-dungeon-catalog', 'missing.json']
        with mock.patch.object(pathlib.Path, 'is_file', side_effect=AssertionError('no export check')):
            with mock.patch.object(pathlib.Path, 'read_text', side_effect=AssertionError('no export read')):
                validate_json_catalogs(command, {'DFO_PVF_CATALOGS': 'characters,dungeons'})
        with contextlib.redirect_stderr(io.StringIO()):
            with self.assertRaises(RuntimeError):
                validate_json_catalogs(command, {'DFO_PVF_CATALOGS': 'characters'})
            with self.assertRaises(RuntimeError):
                validate_json_catalogs(command, {})

    def test_native_gateway_capability_cannot_silently_fall_back(self):
        tree = ast.parse((PROBE / 'channel_probe.py').read_text(encoding='utf-8-sig'))
        node = next(n for n in tree.body if isinstance(n, ast.FunctionDef) and n.name == 'prune_unsupported')
        scope = {'os': os, 'exe_flags': lambda _: {'character-catalog'}}
        exec(compile(ast.fix_missing_locations(ast.Module(body=[node], type_ignores=[])), '<gateway-capability>', 'exec'), scope)
        with mock.patch.dict(os.environ, {'DFO_PVF_CATALOGS': 'characters'}, clear=True):
            with self.assertRaises(RuntimeError):
                scope['prune_unsupported'](['old-server', '-character-catalog', 'legacy.json'])

    def test_native_wait_accepts_readiness_after_old_thirty_second_limit(self):
        with tempfile.TemporaryDirectory() as directory:
            project = pathlib.Path(directory)
            count = 0

            def sleep(_):
                nonlocal count
                count += 1
                if count == 400:
                    session = next((project / 'runtime').iterdir())
                    (session / 'run.json').write_text(json.dumps({'server_pid': 123, 'port': 1}))

            configuration = ({'client_dir': str(project), 'server_binary': 'legacy.exe'}, {},
                             types.SimpleNamespace(hostname='127.0.0.1', port=25438))
            child = mock.Mock()
            child.poll.return_value = None
            with mock.patch.object(launch, 'PROJECT', project), \
                 mock.patch.object(launch, 'configuration', return_value=configuration), \
                 mock.patch.object(launch, 'gateway_configuration', return_value=(project / 'pvf.exe', [], {'DFO_PVF_CATALOGS': 'characters'})), \
                 mock.patch.object(launch, '_ensure_inner_pvf') as prepare, \
                 mock.patch.object(launch, 'start_storage') as storage, \
                 mock.patch.object(launch, 'listening', return_value=False), \
                 mock.patch.object(pathlib.Path, 'is_file', return_value=True), \
                 mock.patch.object(launch.subprocess, 'Popen', return_value=child) as popen, \
                 mock.patch.object(launch.time, 'sleep', side_effect=sleep), \
                 mock.patch.object(sys, 'argv', ['launch_local.py', '--server-only']), \
                 contextlib.redirect_stdout(io.StringIO()):
                launch.main()
            self.assertEqual(count, 400)
            prepare.assert_called_once()
            storage.assert_called_once()
            popen.assert_called_once()
            self.assertEqual(popen.call_args.kwargs['env']['DFO_PVF_CATALOGS'], 'characters')

    def test_check_json_mode_never_starts_storage_or_helper(self):
        configuration = ({'client_dir': str(launch.PROJECT), 'server_binary': 'legacy.exe'}, {},
                         types.SimpleNamespace(hostname='127.0.0.1', port=25438))
        with mock.patch.object(launch, 'configuration', return_value=configuration), \
             mock.patch.object(launch, 'start_storage') as storage, \
             mock.patch.object(launch.subprocess, 'Popen') as popen, \
             mock.patch.object(launch, 'listening', return_value=False), \
             mock.patch.object(pathlib.Path, 'is_file', return_value=True), \
             mock.patch.object(sys, 'argv', ['launch_local.py', '--check', '--server-only', '--json-mode']), \
             contextlib.redirect_stdout(io.StringIO()) as output:
            launch.main()
        storage.assert_not_called()
        popen.assert_not_called()
        self.assertIn('legacy.exe', output.getvalue())
        self.assertIn('JSON / explicit profile', output.getvalue())


if __name__ == '__main__':
    unittest.main()

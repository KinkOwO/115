"""Environment migration checks; no game or database process is started."""
import contextlib
import io
import json
import pathlib
import sys
import tempfile
import unittest
from unittest import mock

import configure_env
import stop_environment
import storage_profile


class StorageDriverTests(unittest.TestCase):
    """The Python half of the engine-selection rule.

    Same table as internal/database/engine_selection_test.go (Go) and the
    launch_local.py cases in server/work/dfo-lan/scripts/test_postgres_storage.py.
    The mixed case is the regression: a PostgreSQL DSN must outrank a leftover
    sqlite_path, or the server opens an empty SQLite file while PostgreSQL runs
    (2026-10-05, pgsql 端无法登录)."""

    def test_driver_table(self):
        cases = [
            ('explicit sqlite wins', {'driver': 'sqlite'}, 'sqlite'),
            ('explicit postgres wins', {'driver': 'postgres'}, 'postgres'),
            ('driver is case and space insensitive', {'driver': ' SQLite '}, 'sqlite'),
            (
                'a DSN outranks a leftover sqlite_path',
                {'postgres_dsn': 'postgres://u@127.0.0.1:25438/dfo_lan', 'sqlite_path': 'leftover.sqlite3'},
                'postgres',
            ),
            ('DSN alone selects postgres', {'postgres_dsn': 'postgres://u@127.0.0.1:25438/dfo_lan'}, 'postgres'),
            ('sqlite_path alone selects sqlite', {'sqlite_path': 'dfolan.sqlite3'}, 'sqlite'),
            ('neither stays on postgres', {}, 'postgres'),
            ('blank DSN does not count as a DSN', {'postgres_dsn': '  ', 'sqlite_path': 'dfolan.sqlite3'}, 'sqlite'),
        ]
        for name, cfg, want in cases:
            with self.subTest(name):
                self.assertEqual(storage_profile.storage_driver(cfg), want)
                # stop_environment must answer identically, not keep its own copy.
                self.assertEqual(stop_environment.storage_driver(cfg), want)

    def test_stop_leaves_a_sqlite_profile_alone(self):
        """A sqlite_path-only profile must not make the stop script kill PostgreSQL."""
        cfg = {'sqlite_path': 'dfolan.sqlite3'}
        with mock.patch.object(stop_environment, 'load_storage_config', return_value=cfg), \
             mock.patch.object(stop_environment, 'stop_wireprobe'), \
             mock.patch.object(stop_environment, 'listening', return_value=False), \
             mock.patch.object(stop_environment.subprocess, 'run') as run, \
             contextlib.redirect_stdout(io.StringIO()) as output:
            stop_environment.stop_postgres(cfg)
        run.assert_not_called()
        self.assertIn('sqlite profile', output.getvalue())

    def test_reconfigure_drops_sqlite_keys_from_a_postgres_profile(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            client = root / 'client'
            client.mkdir()
            for name in ('DFO.exe', 'Script.pvf', 'sk.dat'):
                (client / name).touch()
            storage = root / 'server/work/dfo-lan/runtime/storage'
            storage.mkdir(parents=True)
            saved = {
                'postgres_dsn': 'postgres://owner:fixture@127.0.0.1:25438/dfo_lan',
                'sqlite_path': 'C:/somewhere/leftover.sqlite3',
                'sqlite_busy_timeout_ms': 5000,
            }
            local = storage / 'local.json'
            local.write_text(json.dumps(saved), encoding='utf-8')
            with mock.patch.object(configure_env, 'get_root_dir', return_value=root), \
                 mock.patch.object(sys, 'argv', ['configure_env.py', '--non-interactive', '--client-dir', str(client)]), \
                 contextlib.redirect_stdout(io.StringIO()):
                configure_env.main()
            updated = json.loads(local.read_text(encoding='utf-8'))
            self.assertNotIn('sqlite_path', updated)
            self.assertNotIn('sqlite_busy_timeout_ms', updated)
            self.assertEqual(updated['postgres_dsn'], saved['postgres_dsn'])
            self.assertEqual(storage_profile.storage_driver(updated), 'postgres')

    def test_reconfigure_keeps_a_sqlite_profile_sqlite(self):
        """A config that names only sqlite_path must not be rewritten into PostgreSQL."""
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            client = root / 'client'
            client.mkdir()
            for name in ('DFO.exe', 'Script.pvf', 'sk.dat'):
                (client / name).touch()
            storage = root / 'server/work/dfo-lan/runtime/storage'
            storage.mkdir(parents=True)
            saved = {'sqlite_path': str(storage / 'dfolan.sqlite3')}
            local = storage / 'local.json'
            local.write_text(json.dumps(saved), encoding='utf-8')
            with mock.patch.object(configure_env, 'get_root_dir', return_value=root), \
                 mock.patch.object(sys, 'argv', ['configure_env.py', '--non-interactive', '--client-dir', str(client)]), \
                 contextlib.redirect_stdout(io.StringIO()):
                configure_env.main()
            updated = json.loads(local.read_text(encoding='utf-8'))
            self.assertEqual(updated['sqlite_path'], saved['sqlite_path'])
            self.assertNotIn('postgres_dsn', updated)
            self.assertEqual(storage_profile.storage_driver(updated), 'sqlite')


class EnvironmentStorageTests(unittest.TestCase):
    def test_reconfigure_preserves_pg_credentials_and_discards_retired_fields(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            client = root / 'client'
            client.mkdir()
            for name in ('DFO.exe', 'Script.pvf', 'sk.dat'):
                (client / name).touch()
            storage = root / 'server/work/dfo-lan/runtime/storage'
            storage.mkdir(parents=True)
            saved = {
                'postgres_dsn': 'postgres://owner:fixture@127.0.0.1:25438/dfo_lan',
                'max_connections': 7,
                'postgres_schema': 'existing',
                'redis_address': 'invalid',
                'redis_password': 'obsolete',
                'redis_bin': 'missing',
                'redis_prefix': 'old:',
                'redis_pid': 99,
            }
            local = storage / 'local.json'
            local.write_text(json.dumps(saved), encoding='utf-8')
            with mock.patch.object(configure_env, 'get_root_dir', return_value=root), \
                 mock.patch.object(sys, 'argv', ['configure_env.py', '--non-interactive', '--client-dir', str(client)]), \
                 contextlib.redirect_stdout(io.StringIO()):
                configure_env.main()
            updated = json.loads(local.read_text(encoding='utf-8'))
            self.assertEqual(updated['postgres_dsn'], saved['postgres_dsn'])
            self.assertEqual(updated['max_connections'], saved['max_connections'])
            self.assertEqual(updated['postgres_schema'], saved['postgres_schema'])
            self.assertFalse(any(key.startswith('redis_') for key in updated))

    def test_stop_ignores_retired_endpoints_and_checks_pg_and_gateway(self):
        cfg = {'redis_address': 'invalid', 'redis_bin': 'missing'}
        with mock.patch.object(stop_environment, 'load_storage_config', return_value=cfg), \
             mock.patch.object(stop_environment, 'stop_wireprobe') as game, \
             mock.patch.object(stop_environment, 'stop_postgres') as postgres, \
             mock.patch.object(stop_environment, 'listening', return_value=False) as listening, \
             mock.patch.object(stop_environment.subprocess, 'run') as run, \
             contextlib.redirect_stdout(io.StringIO()) as output:
            stop_environment.main()
        game.assert_called_once_with()
        postgres.assert_called_once_with(cfg)
        self.assertEqual(listening.call_args_list, [mock.call('127.0.0.1', 25438), mock.call('127.0.0.1', 7001)])
        run.assert_not_called()
        self.assertIn('Environment fully stopped.', output.getvalue())


if __name__ == '__main__':
    unittest.main()

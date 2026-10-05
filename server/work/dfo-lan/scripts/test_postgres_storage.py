"""Storage bootstrap/launch regression checks with no real database processes."""
import json
import pathlib
import tempfile
import types
import unittest
from unittest import mock

import bootstrap_local as bootstrap
import launch_local as launch


class PostgresStorageTests(unittest.TestCase):
    def test_configuration_accepts_pg_only_and_ignores_legacy_fields(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            (root / 'launcher.local.json').write_text(json.dumps({'client_dir': 'client'}))
            storage = root / 'storage'
            storage.mkdir()
            for legacy in ({}, {'redis_address': 'invalid', 'redis_bin': 'missing', 'redis_password': 'obsolete'}):
                cfg = dict(legacy, postgres_dsn='postgres://user:pass@127.0.0.1:25438/dfo_lan')
                (storage / 'local.json').write_text(json.dumps(cfg))
                with mock.patch.object(launch, 'ROOT', root), mock.patch.object(launch, 'STORAGE', storage):
                    local, parsed, pg = launch.configuration()
                self.assertEqual(local['client_dir'], 'client')
                self.assertEqual(parsed, cfg)
                self.assertEqual((pg.hostname, pg.port), ('127.0.0.1', 25438))

    def test_running_postgres_requires_no_other_process_or_files(self):
        pg = types.SimpleNamespace(hostname='127.0.0.1', port=25438)
        with mock.patch.object(launch, 'listening', return_value=True) as listening, \
             mock.patch.object(launch.subprocess, 'run') as run, \
             mock.patch.object(launch.subprocess, 'Popen') as popen:
            launch.start_storage({}, pg)
        listening.assert_called_once_with('127.0.0.1', 25438)
        run.assert_not_called()
        popen.assert_not_called()

    def test_offline_postgres_keeps_existing_data_and_start_options(self):
        with tempfile.TemporaryDirectory() as directory:
            storage = pathlib.Path(directory)
            data = storage / 'pgdata'
            data.mkdir()
            (data / 'PG_VERSION').write_text('16')
            pgctl = storage / 'pg_ctl.exe'
            pgctl.touch()
            cfg = {'postgres_data': str(data), 'postgres_bin': str(storage)}
            pg = types.SimpleNamespace(hostname='127.0.0.1', port=25438)
            with mock.patch.object(launch, 'STORAGE', storage), \
                 mock.patch.object(launch, 'listening', side_effect=[False, True]), \
                 mock.patch.object(launch.subprocess, 'run', return_value=types.SimpleNamespace(returncode=0)) as run, \
                 mock.patch.object(launch.subprocess, 'Popen') as popen:
                launch.start_storage(cfg, pg)
            self.assertEqual(run.call_args.args[0], [str(pgctl), '-D', str(data), '-l', str(storage / 'postgres.log'), '-w', '-t', '30', 'start'])
            self.assertEqual((data / 'PG_VERSION').read_text(), '16')
            popen.assert_not_called()

    def test_bootstrap_creates_pg_only_configuration(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            pgbin = root / 'bin'
            pgbin.mkdir()
            for name in ('initdb.exe', 'pg_ctl.exe', 'createdb.exe'):
                (pgbin / name).touch()
            runtime = root / 'storage'

            def execute(command, **options):
                if pathlib.Path(command[0]).name == 'initdb.exe':
                    data = pathlib.Path(command[command.index('-D') + 1])
                    data.mkdir()
                    (data / 'postgresql.conf').touch()
                return types.SimpleNamespace(returncode=0, stdout=b'', stderr=b'')

            with mock.patch.object(bootstrap.socket, 'socket') as connection, \
                 mock.patch.object(bootstrap.subprocess, 'run', side_effect=execute) as run, \
                 mock.patch.object(bootstrap.subprocess, 'Popen') as popen:
                result = bootstrap.initialize(pgbin, 25438, runtime)
            connection.return_value.__enter__.return_value.bind.assert_called_once_with(('127.0.0.1', 25438))
            self.assertEqual([pathlib.Path(call.args[0][0]).name for call in run.call_args_list], ['initdb.exe', 'pg_ctl.exe', 'createdb.exe'])
            cfg = json.loads((runtime / 'local.json').read_text())
            self.assertEqual(set(cfg), {'postgres_dsn', 'max_connections', 'postgres_bin', 'postgres_data'})
            self.assertEqual(set(result), {'config_path', 'postgres_port'})
            self.assertFalse((runtime / 'initdb-password.tmp').exists())
            popen.assert_not_called()

    def test_bootstrap_refuses_existing_saves_before_any_process(self):
        for existing in ('local.json', 'pgdata'):
            with self.subTest(existing=existing), tempfile.TemporaryDirectory() as directory:
                runtime = pathlib.Path(directory)
                path = runtime / existing
                path.write_text('unchanged') if existing.endswith('.json') else path.mkdir()
                with mock.patch.object(bootstrap.subprocess, 'run') as run, \
                     mock.patch.object(bootstrap.socket, 'socket') as connection:
                    with self.assertRaisesRegex(SystemExit, 'already exists'):
                        bootstrap.initialize('missing', 25438, runtime)
                run.assert_not_called()
                connection.assert_not_called()
                self.assertTrue(path.exists())
                if path.is_file():
                    self.assertEqual(path.read_text(), 'unchanged')


    def test_storage_driver_matches_the_go_rule(self):
        """Same table as internal/database/engine_selection_test.go (Go) and
        scripts/test_environment_storage.py. The mixed case is the 2026-10-05 regression:
        a DSN must outrank a leftover sqlite_path, or PostgreSQL gets started while the
        server opens an empty SQLite file (pgsql 端无法登录)."""
        cases = [
            ('explicit sqlite', {'driver': 'sqlite'}, 'sqlite'),
            ('explicit postgres', {'driver': 'postgres'}, 'postgres'),
            ('case and space insensitive', {'driver': ' SQLite '}, 'sqlite'),
            ('an unknown driver is reported as written', {'driver': 'mysql'}, 'mysql'),
            ('DSN outranks leftover sqlite_path',
             {'postgres_dsn': 'postgres://u@127.0.0.1:25438/dfo_lan', 'sqlite_path': 'leftover.sqlite3'}, 'postgres'),
            ('DSN alone', {'postgres_dsn': 'postgres://u@127.0.0.1:25438/dfo_lan'}, 'postgres'),
            ('sqlite_path alone', {'sqlite_path': 'dfolan.sqlite3'}, 'sqlite'),
            ('neither', {}, 'postgres'),
        ]
        for name, cfg, want in cases:
            with self.subTest(name):
                self.assertEqual(launch.storage_driver(cfg), want)

    def test_sqlite_only_profile_launches_without_postgresql(self):
        """A config the server opens as SQLite must not send the launcher to PostgreSQL,
        and must not die with a KeyError on the missing postgres_dsn."""
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            (root / 'launcher.local.json').write_text(json.dumps({'client_dir': 'client'}))
            storage = root / 'storage'
            storage.mkdir()
            cfg = {'sqlite_path': str(storage / 'dfolan.sqlite3')}
            (storage / 'local.json').write_text(json.dumps(cfg))
            with mock.patch.object(launch, 'ROOT', root), mock.patch.object(launch, 'STORAGE', storage):
                local, parsed, pg = launch.configuration()
            self.assertEqual(parsed, cfg)
            self.assertIsNone(pg)

    def test_postgres_profile_without_a_dsn_reports_instead_of_crashing(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            (root / 'launcher.local.json').write_text(json.dumps({'client_dir': 'client'}))
            storage = root / 'storage'
            storage.mkdir()
            (storage / 'local.json').write_text(json.dumps({'driver': 'postgres', 'max_connections': 12}))
            with mock.patch.object(launch, 'ROOT', root), mock.patch.object(launch, 'STORAGE', storage):
                with self.assertRaisesRegex(RuntimeError, 'requires postgres_dsn'):
                    launch.configuration()


if __name__ == '__main__':
    unittest.main()

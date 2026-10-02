"""GM storage checks must work without additional storage services."""
import contextlib
import io
import json
import pathlib
import sys
import tempfile
import types
import unittest
from unittest import mock

import gmweb


class PostgresStorageTests(unittest.TestCase):
    def test_online_postgres_accepts_new_and_legacy_configuration(self):
        for legacy in ({}, {'redis_address': 'invalid', 'redis_bin': 'missing'}):
            cfg = dict(legacy, postgres_dsn='postgres://user:pass@127.0.0.1:25438/dfo_lan')
            with mock.patch.object(gmweb, 'listening', return_value=True) as listening, \
                 mock.patch.object(gmweb.subprocess, 'run') as run, \
                 mock.patch.object(gmweb.subprocess, 'Popen') as popen:
                gmweb.start_storage(pathlib.Path('missing/local.json'), cfg)
            listening.assert_called_once_with('127.0.0.1', 25438)
            run.assert_not_called()
            popen.assert_not_called()

    def test_offline_postgres_uses_only_existing_pg_data(self):
        with tempfile.TemporaryDirectory() as directory:
            storage = pathlib.Path(directory)
            data = storage / 'pgdata'
            data.mkdir()
            (data / 'PG_VERSION').write_text('16')
            pgctl = storage / 'pg_ctl.exe'
            pgctl.touch()
            cfg = {'postgres_dsn': 'postgres://user:pass@127.0.0.1:25438/dfo_lan',
                   'postgres_data': str(data), 'postgres_bin': str(storage)}
            with mock.patch.object(gmweb, 'listening', side_effect=[False, True]), \
                 mock.patch.object(gmweb.subprocess, 'run', return_value=types.SimpleNamespace(returncode=0)) as run, \
                 mock.patch.object(gmweb.subprocess, 'Popen') as popen:
                gmweb.start_storage(storage / 'local.json', cfg)
            self.assertEqual(run.call_args.args[0], [str(pgctl), '-D', str(data), '-l', str(storage / 'postgres.log'), '-w', '-t', '30', 'start'])
            self.assertEqual((data / 'PG_VERSION').read_text(), '16')
            popen.assert_not_called()

    def test_check_accepts_pg_only_without_starting_anything(self):
        with tempfile.TemporaryDirectory() as directory:
            storage_file = pathlib.Path(directory) / 'local.json'
            storage_file.write_text(json.dumps({'postgres_dsn': 'postgres://user:pass@127.0.0.1:25438/dfo_lan'}))
            with mock.patch.object(sys, 'argv', ['gmweb.py', '--check', '--storage', str(storage_file)]), \
                 mock.patch.object(pathlib.Path, 'is_file', return_value=True), \
                 mock.patch.object(gmweb, 'listening', return_value=True) as listening, \
                 mock.patch.object(gmweb, 'start_storage') as storage, \
                 mock.patch.object(gmweb.subprocess, 'run') as run, \
                 mock.patch.object(gmweb.webbrowser, 'open') as browser, \
                 contextlib.redirect_stdout(io.StringIO()) as output:
                gmweb.main()
            listening.assert_called_once_with('127.0.0.1', 25438)
            storage.assert_not_called()
            run.assert_not_called()
            browser.assert_not_called()
            self.assertIn('PostgreSQL: True', output.getvalue())


if __name__ == '__main__':
    unittest.main()

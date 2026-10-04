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

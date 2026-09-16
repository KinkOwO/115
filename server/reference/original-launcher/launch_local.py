"""Start the existing local test environment without initializing storage."""
import argparse
import ctypes
import datetime
import json
import pathlib
import socket
import subprocess
import sys
import time
from urllib.parse import urlparse

ROOT = pathlib.Path(__file__).resolve().parent.parent
STORAGE = ROOT / 'runtime/storage'
PROBE = ROOT.parent / 'dfo_probe_tools/channel_probe.py'
REDIS = pathlib.Path('E:/codex/2026-09-09/mwng5er8gq5osnw/outputs/street-three-kingdoms-source/server/vendor/redis/Redis-8.10.1-Windows-x64-cygwin/redis-server.exe')


def listening(host, port):
    try:
        with socket.create_connection((host, port), timeout=0.5):
            return True
    except OSError:
        return False


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--check', action='store_true', help='Verify paths only; do not start anything')
    parser.add_argument('--storage-only', action='store_true', help='Start existing project storage without opening the client')
    args = parser.parse_args()
    config = json.loads((STORAGE / 'local.json').read_text(encoding='utf-8'))
    pgctl = pathlib.Path(config['postgres_bin']) / 'pg_ctl.exe'
    pgdata = pathlib.Path(config['postgres_data']).resolve()
    if pgdata != (STORAGE / 'pgdata').resolve():
        raise RuntimeError('PostgreSQL data path is outside this project; startup stopped.')
    required = [PROBE, PROBE.parent / 'probe.exe', ROOT.parent / 'dfo_probe_client/DFO.exe',
                ROOT / 'bin/wireprobe-dungeon35.exe', pgctl, pgdata / 'PG_VERSION',
                REDIS, STORAGE / 'redis.conf', ROOT / 'configs/channel.local34.json',
                ROOT / 'configs/inventory.next29.json', ROOT / 'configs/equipment.current35.json',
                ROOT / 'configs/equipment-wear.current35.json', ROOT / 'configs/account-options.current35.json',
                ROOT / 'bin/wireprobe-dungeon36.exe', ROOT / 'configs/drop.current36.json',
                ROOT / 'configs/tutorial-routes.current35.json', ROOT / 'configs/tutorial-dungeons.current36.json',
                ROOT / 'bin/wireprobe-dungeon37.exe', ROOT / 'configs/equipment.current37.json']
    for path in required:
        if not path.exists():
            raise RuntimeError('Required file missing: ' + str(path))
    pg = urlparse(config['postgres_dsn'])
    redis_host, redis_port = config['redis_address'].rsplit(':', 1)
    if pg.hostname != '127.0.0.1' or redis_host != '127.0.0.1':
        raise RuntimeError('This launcher supports project-local loopback storage only.')
    if args.check:
        print('Launcher paths OK. Check mode: nothing started or changed.')
        print('PostgreSQL listening:', listening(pg.hostname, pg.port))
        print('Redis listening:', listening(redis_host, int(redis_port)))
        return
    if not ctypes.windll.shell32.IsUserAnAdmin():
        raise RuntimeError('Right-click Start-DFO.cmd and choose Run as administrator.')
    if not args.storage_only and listening('127.0.0.1', 7001):
        raise RuntimeError('Port 7001 is already in use. Close the existing DFO test session before starting again.')
    flags = subprocess.CREATE_NO_WINDOW
    if not listening(pg.hostname, pg.port):
        with (STORAGE / 'launcher-postgres.log').open('ab') as log:
            result = subprocess.run([str(pgctl), '-D', str(pgdata), '-l', str(STORAGE / 'postgres.log'),
                                     '-w', '-t', '30', 'start'], stdout=log, stderr=log,
                                    creationflags=flags, timeout=40)
        if result.returncode or not listening(pg.hostname, pg.port):
            raise RuntimeError('PostgreSQL startup failed; see runtime/storage/launcher-postgres.log')
    if not listening(redis_host, int(redis_port)):
        with (STORAGE / 'redis.log').open('ab') as log:
            redis = subprocess.Popen([str(REDIS), 'redis.conf'], cwd=STORAGE,
                                     stdout=log, stderr=log, creationflags=flags)
        for _ in range(50):
            if listening(redis_host, int(redis_port)):
                break
            if redis.poll() is not None:
                raise RuntimeError('Redis startup failed; see runtime/storage/redis.log')
            time.sleep(0.1)
        else:
            raise RuntimeError('Redis startup timed out; see runtime/storage/redis.log')
    if args.storage_only:
        print('Existing project PostgreSQL and Redis are ready; no client started.')
        return
    stamp = datetime.datetime.now().strftime('%Y%m%d_%H%M%S_%f')
    tag = 'roles_persist_select_actor_town_world_live_detail_dungeon_manual_' + stamp + '_next37'
    out = ROOT / 'runtime' / tag
    out.mkdir(parents=True)
    with (out / 'helper.out').open('wb') as stdout, (out / 'helper.err').open('wb') as stderr:
        helper = subprocess.Popen([sys.executable, str(PROBE), tag, 'interactive'], cwd=ROOT.parent.parent,
                                  stdin=subprocess.DEVNULL, stdout=stdout, stderr=stderr, creationflags=flags)
    for _ in range(100):
        if helper.poll() is not None:
            raise RuntimeError('Launch helper stopped; see ' + str(out / 'helper.err'))
        if (out / 'run.json').exists():
            print('Local test build 37 launch requested. Select the local server and channel, then your character.')
            print('Logs: ' + str(out))
            return
        time.sleep(0.1)
    raise RuntimeError('Startup is taking longer than expected. Check logs before retrying: ' + str(out))


if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        print('ERROR: ' + str(error), file=sys.stderr)
        sys.exit(1)

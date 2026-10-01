"""Start the gmweb GM UI against an existing DFO storage, then open the browser.

Standalone package: the exe, its catalogs and a trimmed Python live inside this
package. The database belongs to the recipient's own D:\\115us environment; the
default --storage points there, override it if the layout differs.
"""
import argparse, json, pathlib, socket, subprocess, sys, threading, time, webbrowser
from urllib.parse import urlparse

ROOT = pathlib.Path(__file__).resolve().parent.parent
BIN = ROOT / 'bin'
CONFIGS = ROOT / 'configs'
GMWEB = BIN / 'gmweb.exe'
FLAGS = getattr(subprocess, 'CREATE_NO_WINDOW', 0)


def listening(host, port):
    try:
        with socket.create_connection((host, port), timeout=.5):
            return True
    except OSError:
        return False


def load_storage(path):
    return json.loads(pathlib.Path(path).read_text(encoding='utf-8-sig'))


def start_storage(storage_file, cfg):
    storage_dir = storage_file.resolve().parent
    pg = urlparse(cfg['postgres_dsn'])
    rh, rp = cfg['redis_address'].rsplit(':', 1)
    if not listening(pg.hostname, pg.port):
        data = pathlib.Path(cfg.get('postgres_data', '')).resolve()
        pgctl = pathlib.Path(cfg.get('postgres_bin', '')) / 'pg_ctl.exe'
        if not (data / 'PG_VERSION').exists() or not pgctl.exists():
            raise RuntimeError('PostgreSQL offline and no usable postgres_bin/postgres_data in local.json; start the DFO server first.')
        with (storage_dir / 'launcher-postgres.log').open('ab') as log:
            r = subprocess.run([str(pgctl), '-D', str(data), '-l', str(storage_dir / 'postgres.log'),
                                '-w', '-t', '30', 'start'], stdout=log, stderr=log, creationflags=FLAGS, timeout=40)
        if r.returncode or not listening(pg.hostname, pg.port):
            raise RuntimeError('PostgreSQL did not start; inspect launcher-postgres.log.')
    if not listening(rh, rp):
        redis = pathlib.Path(cfg.get('redis_bin', '')) / 'redis-server.exe'
        conf = storage_dir / 'redis.conf'
        if not redis.exists() or not conf.exists():
            raise RuntimeError('Redis offline and no usable redis_bin/redis.conf; start the DFO server first.')
        with (storage_dir / 'redis.log').open('ab') as log:
            child = subprocess.Popen([str(redis), str(conf)], cwd=storage_dir, stdout=log, stderr=log, creationflags=FLAGS)
        for _ in range(100):
            if listening(rh, rp):
                return
            if child.poll() is not None:
                break
            time.sleep(.1)
        raise RuntimeError('Redis did not start; inspect redis.log.')


def catalog_args(args=None):
    if args is not None and args.catalog_source == 'pvf':
        if not args.pvf_archive or not args.pvf_source_checksum:
            raise ValueError('PVF mode needs an explicit inner archive and exact save-source SHA256.')
        return [
            '-catalog-source', 'pvf', '-pvf-archive', str(pathlib.Path(args.pvf_archive).resolve()),
            '-pvf-source-checksum', args.pvf_source_checksum,
            '-pvf-drop-policy', str(pathlib.Path(args.pvf_drop_policy).resolve()),
            '-bag-rules', str(CONFIGS / 'inventory.current37.json'),
            '-names-client', str(CONFIGS / 'names.client.json'),
        ]
    return [
        '-loot-catalog', str(CONFIGS / 'loot.next25.json'),
        '-bag-rules', str(CONFIGS / 'inventory.current37.json'),
        '-equipment-catalog', str(CONFIGS / 'equipment.current37.json'),
        '-character-catalog', str(CONFIGS / 'characters.next25.json'),
        '-progression-catalog', str(CONFIGS / 'progression.next25.json'),
        '-progression-rules', str(CONFIGS / 'experience.compat90.json'),
        '-item-index', str(CONFIGS / 'items.index.json'),
    ]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--storage', default=r'D:\115us\server\work\dfo-lan\runtime\storage\local.json',
                        help='path to the DFO storage local.json')
    parser.add_argument('--check', action='store_true', help='read-only dependency check; starts nothing')
    parser.add_argument('--listen', default='127.0.0.1:28080', help='gmweb HTTP listen address')
    parser.add_argument('--no-browser', action='store_true', help='do not open the browser')
    parser.add_argument('--gmweb-binary', default=str(GMWEB), help='explicit candidate program; default keeps the existing release')
    parser.add_argument('--catalog-source', choices=('json', 'pvf'), default='json')
    parser.add_argument('--pvf-archive', default='')
    parser.add_argument('--pvf-source-checksum', default='')
    parser.add_argument('--pvf-drop-policy', default=str(ROOT.parent / 'server' / 'work' / 'dfo-lan' / 'configs' / 'pvf-drop-policy.json'))
    args = parser.parse_args()

    gmweb = pathlib.Path(args.gmweb_binary).resolve()
    source_args = catalog_args(args)
    if not gmweb.is_file():
        raise RuntimeError('Missing bin/gmweb.exe; this package is incomplete.')

    storage_file = pathlib.Path(args.storage)
    if args.check and args.catalog_source == 'pvf':
        # The candidate exits before reading storage or starting any processes.
        result = subprocess.run([str(gmweb), '-root', str(ROOT.parent), '-storage', str(storage_file.resolve()),
                                '-check-catalogs'] + source_args, cwd=ROOT, capture_output=True,
                                text=True, encoding='utf-8', creationflags=FLAGS)
        print(result.stdout, end='')
        if result.stderr:
            print(result.stderr, end='', file=sys.stderr)
        if result.returncode:
            raise RuntimeError('Native catalog check failed (exit %d).' % result.returncode)
        return
    if args.check:
        print('gmweb.exe:' , True, 'configs:', all((CONFIGS / f).is_file() for f in (
            'loot.next25.json', 'inventory.current37.json', 'equipment.current37.json',
            'characters.next25.json', 'progression.next25.json', 'experience.compat90.json',
            'items.index.json')))
        if not storage_file.is_file():
            print('storage missing:', storage_file.resolve())
            return
        cfg = load_storage(storage_file)
        pg = urlparse(cfg['postgres_dsn'])
        rh, rp = cfg['redis_address'].rsplit(':', 1)
        print('storage:', storage_file.resolve())
        print('PostgreSQL:', listening(pg.hostname, pg.port), 'Redis:', listening(rh, rp))
        return

    cfg = load_storage(storage_file)
    start_storage(storage_file, cfg)

    host, port = args.listen.rsplit(':', 1)
    port = int(port)
    if not args.no_browser:
        def open_when_ready():
            for _ in range(50):
                if listening(host, port):
                    break
                time.sleep(.1)
            webbrowser.open('http://' + args.listen)
        threading.Thread(target=open_when_ready, daemon=True).start()

    cmd = [str(gmweb), '-root', str(ROOT.parent), '-listen', args.listen, '-storage', str(storage_file.resolve()), '-open=false'] + source_args
    print('GM web: http://' + args.listen + '  (backend only, browser opens on dashboard 28081)', flush=True)
    subprocess.run(cmd, cwd=ROOT)


if __name__ == '__main__':
    try:
        main()
    except Exception as exc:
        print('ERROR:', exc, file=sys.stderr)
        sys.exit(1)

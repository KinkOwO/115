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


def catalog_args(args):
    if args.catalog_source != 'pvf':
        raise ValueError('GM game catalogs require PVF; the JSON content source is retired.')
    if not args.pvf_archive:
        raise ValueError('An inner PVF archive path is required.')
    return [
        '-catalog-source', 'pvf', '-pvf-archive', str(pathlib.Path(args.pvf_archive).resolve()),
        '-pvf-source-checksum', args.pvf_source_checksum,
        '-pvf-drop-policy', str(pathlib.Path(args.pvf_drop_policy).resolve()),
        '-bag-rules', str(CONFIGS / 'inventory.current37.json'),
        '-names-client', str(CONFIGS / 'names.client.json'),
    ]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--storage', default=str(ROOT.parent / 'server' / 'work' / 'dfo-lan' / 'runtime' / 'storage' / 'local.json'),
                        help='path to the DFO storage local.json')
    parser.add_argument('--check', action='store_true', help='read-only dependency check; starts nothing')
    parser.add_argument('--listen', default='127.0.0.1:28080', help='gmweb HTTP listen address')
    parser.add_argument('--no-browser', action='store_true', help='do not open the browser')
    parser.add_argument('--gmweb-binary', default=str(GMWEB), help='explicit candidate program; default keeps the existing release')
    parser.add_argument('--catalog-source', choices=('pvf',), default='pvf')
    parser.add_argument('--pvf-archive', default='', help='inner PVF; default follows the storage module')
    parser.add_argument('--pvf-source-checksum', default='')
    parser.add_argument('--pvf-drop-policy', default='', help='default follows the storage module')
    args = parser.parse_args()

    storage_file = pathlib.Path(args.storage).resolve()
    module = storage_file.parent.parent.parent
    if not args.pvf_archive:
        args.pvf_archive = str(module.parent / 'client-build' / 'Script.inner.pvf')
    if not args.pvf_drop_policy:
        args.pvf_drop_policy = str(module / 'configs' / 'pvf-drop-policy.json')
    gmweb = pathlib.Path(args.gmweb_binary).resolve()
    source_args = catalog_args(args)
    if not gmweb.is_file():
        raise RuntimeError('Missing bin/gmweb.exe; this package is incomplete.')

    if args.check:
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

"""Compare D:\115us\client against the pristine DFO.zip (entries have DFO/ prefix).
Match by relative path + CRC32 + size. List changed / added files."""
import json
import pathlib
import zlib
import zipfile

ZIP_PATH = r'D:\游戏\端游\DNF\115脱壳\115us千海天\DFO.zip'
CLIENT = pathlib.Path(r'D:\115us\client')
REPORT = pathlib.Path(r'D:\115us\server\work\patch_diff.json')

# Runtime-generated files that must never be treated as patch content.
SKIP_NAMES = {
    'cef.log', 'debug.log', 'Patch.trc',
    'DFO_original.exe', 'Script.2429-backup.pvf',
}
SKIP_SUFFIX = ('.log',)


def skip(rel_posix: str) -> bool:
    name = rel_posix.rsplit('/', 1)[-1]
    if name in SKIP_NAMES:
        return True
    if rel_posix.startswith('BlackCipher/') and name.endswith('.log'):
        return True
    return False


def real_name(info):
    raw = info.filename
    if info.flag_bits & 0x800:
        return raw
    try:
        return raw.encode('cp437').decode('gbk')
    except Exception:
        return raw


# 1. zip index
zf = zipfile.ZipFile(ZIP_PATH)
zip_map = {}
for info in zf.infolist():
    if info.is_dir():
        continue
    name = real_name(info).replace('\\', '/')
    if name.startswith('DFO/'):
        name = name[4:]
    zip_map[name] = (info.CRC & 0xffffffff, info.file_size)
print('zip files:', len(zip_map))

# 2. client index (crc32 over every file, ~51GB, takes a few minutes)
client_map = {}
for path in sorted(CLIENT.rglob('*')):
    if not path.is_file():
        continue
    rel = path.relative_to(CLIENT).as_posix()
    crc = 0
    with path.open('rb') as fh:
        while True:
            chunk = fh.read(4 * 1024 * 1024)
            if not chunk:
                break
            crc = zlib.crc32(chunk, crc)
    client_map[rel] = (crc & 0xffffffff, path.stat().st_size)
print('client files:', len(client_map))

changed, added, skipped = [], [], []
for rel, (crc, size) in client_map.items():
    if skip(rel):
        skipped.append(rel)
        continue
    if rel not in zip_map:
        added.append(rel)
    elif zip_map[rel] != (crc, size):
        changed.append({'path': rel,
                        'zip_crc': f'{zip_map[rel][0]:08x}', 'zip_size': zip_map[rel][1],
                        'new_crc': f'{crc:08x}', 'new_size': size})

# also report files in zip but missing in client (informational)
missing = sorted(set(zip_map) - set(client_map))

REPORT.write_text(json.dumps(
    {'changed': changed, 'added': sorted(added),
     'missing_in_client': missing, 'skipped': sorted(skipped)},
    ensure_ascii=False, indent=1), encoding='utf-8')

print('\n=== CHANGED (', len(changed), ') ===')
for e in changed:
    print(f"{e['path']}  {e['zip_size']}->{e['new_size']}  {e['zip_crc']} -> {e['new_crc']}")
print('\n=== ADDED (', len(added), ') ===')
for p in added:
    print(p)
print('\n=== MISSING IN CLIENT (', len(missing), ') ===')
for p in missing[:20]:
    print(p)
print('\n=== SKIPPED runtime/backup (', len(skipped), ') ===')
for p in skipped:
    print(p)
print('\nreport ->', REPORT)

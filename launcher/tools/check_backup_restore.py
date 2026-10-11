"""Private SQLite integration only. Production port protection is exercised before the test-only port overlay."""
from pathlib import Path
import argparse,subprocess,json,sqlite3,hashlib,os,shutil,uuid
from contextlib import closing
p=argparse.ArgumentParser();p.add_argument('--fixture',type=Path,required=True);p.add_argument('--production-tool',type=Path,required=True);p.add_argument('--test-tool',type=Path,required=True);p.add_argument('--production-success',action='store_true');a=p.parse_args()
W=Path(__file__).resolve().parents[1];E=W/'evidence';d=json.loads(a.fixture.read_text('utf8'));root=Path(d['root']).resolve();db=Path(d['database']).resolve()
assert (root/'ISOLATED-GUI-TEST-FIXTURE').exists() and root.is_relative_to(E.resolve()) and db.is_relative_to(root)
cfg=root/'server/work/dfo-lan/runtime/storage/local.json';config=cfg.read_bytes();checks=[];sha=lambda p:hashlib.file_digest(p.open('rb'),'sha256').hexdigest()
def check(name,value):
 assert value,name
 checks.append(dict(name=name,passed=True))
def run(source,action,name=None,dry=False,production=False,ok=True):
 argv=[str(a.production_tool if production else a.test_tool),'storage-sync','--root',str(root),'--source',source,'--expected-database',str(db),action]
 if name:argv.append(name)
 if dry:argv.append('--dry-run')
 proc=subprocess.run(argv,capture_output=True,creationflags=subprocess.CREATE_NO_WINDOW,env={k:v for k,v in os.environ.items() if not k.upper().startswith('DFO_')},timeout=90)
 rows=[json.loads(line) for line in proc.stdout.decode('utf8','replace').splitlines() if line.startswith('{')]
 assert rows,(proc.returncode,proc.stderr.decode('utf8','replace'))
 result=rows[-1]
 assert (proc.returncode==0 and result.get('ok'))==ok,result.get('error',result)
 assert result['source']==source and os.path.samefile(result['database'],db),result
 return result
def marker(value=None,file=None):
 file=Path(file or db)
 assert file.resolve().is_relative_to(root)
 with closing(sqlite3.connect(str(file))) as connection:
  if value is not None:
   connection.execute('CREATE TABLE IF NOT EXISTS unified_gui_regression_marker (id INTEGER PRIMARY KEY, value TEXT NOT NULL)')
   connection.execute('INSERT OR REPLACE INTO unified_gui_regression_marker VALUES(1,?)',(value,));connection.commit()
  return connection.execute('SELECT value FROM unified_gui_regression_marker WHERE id=1').fetchone()[0]
def backup_file(entry):return Path(entry['directory'])/entry['files'][0]['name']
before=sha(db)
if a.production_success:
 assert sha(a.production_tool)==sha(a.test_tool)
 previous_marker=marker()
 result=run('storage','--backup',production=True)
 check('unmodified production binary performs backup once port is naturally idle',result['ok'] and marker()==previous_marker and cfg.read_bytes()==config)
else:
 blocked=run('storage','--backup',production=True,ok=False)
 check('production binary refuses backup while real gateway port is active', '7001' in blocked.get('error','') and sha(db)==before)
 (E/'S21-production-active-refusal.json').write_text(json.dumps(blocked,ensure_ascii=False,indent=2),'utf8')
marker('source-storage');storage=run('storage','--backup')['backup']
marker('source-gm');gm=run('gm-local','--backup')['backup']
check('both backup sources bind to the actual custom save directory',Path(storage['directory']).parent==db.parent/'backups' and Path(gm['directory']).parent==db.parent/'gm-backups/local')
same='same-name-regression-'+uuid.uuid4().hex[:8]
for row in [storage,gm]:
 source=Path(row['directory']).resolve();target=source.parent/same
 assert source.is_relative_to(root) and target.is_relative_to(root) and not target.exists()
 source.rename(target);row['directory']=str(target);row['name']=same
for source,row in [('storage',storage),('gm-local',gm)]:
 listed=run(source,'--list')['backups'];match=next(x for x in listed if x['name']==same)
 check(source+' same-name entry retains its own source directory',Path(match['directory'])==Path(row['directory']))
marker('before-restore');run('gm-local','--restore',same,dry=True);restored=run('gm-local','--restore',same)['restore']
check('GM-source restore chooses GM data despite identical names',marker()=='source-gm')
check('GM-source restore automatically backed up pre-restore progress',marker(file=backup_file(restored['target_backup']))=='before-restore')
restored=run('storage','--restore',same)['restore']
check('storage-source restore chooses launcher data despite identical names',marker()=='source-storage')
check('storage-source restore automatically backed up previous GM state',marker(file=backup_file(restored['target_backup']))=='source-gm')
file=backup_file(storage);original=file.read_bytes();before=sha(db)
try:
 with file.open('r+b') as f:f.seek(32);f.write(bytes([original[32]^0x80]))
 rejected=run('storage','--restore',same,dry=True,ok=False)
 check('corrupted backup rejected before touching current save',sha(db)==before and 'sha' in rejected.get('error','').lower())
finally:file.write_bytes(original)
manifest=Path(storage['directory'])/'manifest.json';original=manifest.read_bytes();foreign=db.parent/'foreign-test.sqlite3';shutil.copy2(db,foreign)
try:
 value=json.loads(original);value['database']=str(foreign);manifest.write_text(json.dumps(value,ensure_ascii=False),'utf8')
 rejected=run('storage','--restore',same,dry=True,ok=False)
 check('backup bound to another database rejected',sha(db)==before)
finally:manifest.write_bytes(original)
run('storage','--restore','../escape',dry=True,ok=False);check('backup path traversal rejected',sha(db)==before)
lease=Path(str(db)+'.admin-guard');assert not lease.exists();lease.write_text(str(os.getpid()))
try:
 run('storage','--restore',same,dry=True,ok=False);check('active management lease blocks restore even with isolated port',sha(db)==before)
finally:lease.unlink()
check('custom sqlite_path and unknown settings survive every operation',cfg.read_bytes()==config)
with closing(sqlite3.connect(str(db))) as connection:
 check('restored SQLite integrity is valid',connection.execute('PRAGMA integrity_check').fetchone()[0]=='ok')
report=dict(passed=len(checks),checks=checks,fixture=str(root),database=str(db),production_tool_sha256=sha(a.production_tool),test_only_tool_sha256=sha(a.test_tool),test_only_port_overlay=None if a.production_success else 43825,production_success=a.production_success,test_only_tool_distributed=False,game_started=False,real_player_data_accessed=False)
(E/('S21-backup-restore-production-result.json' if a.production_success else 'S21-backup-restore-result.json')).write_text(json.dumps(report,ensure_ascii=False,indent=2),'utf8')
print(f'PASS {len(checks)} two-source backup and restore checks')

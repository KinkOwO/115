"""Two coexisting private roots; compiled Core and production CLI, no game launch."""
from pathlib import Path
import argparse,hashlib,json,os,shutil,sqlite3,subprocess,uuid
from contextlib import closing
p=argparse.ArgumentParser();p.add_argument('--before',action='store_true');a=p.parse_args()
work=Path(__file__).resolve().parents[1];buildroot=work.parent
binarydir=work/'evidence/portability-before' if a.before else work/'dist'
evidence=work/'evidence'/('portability-'+('before-' if a.before else 'fixed-')+uuid.uuid4().hex[:8]);evidence.mkdir()
old=evidence/'原目录 A %23 #';moved=evidence/'复制后 B %25 #';old.mkdir();checks=[]
build=json.loads((binarydir/'build-result.json').read_text('utf-8-sig'));gui='DFO-115US本地版登录器.exe'
sha=lambda x:hashlib.sha256(x.read_bytes()).hexdigest()
def put(path,data):path.parent.mkdir(parents=True,exist_ok=True);path.write_bytes(data if isinstance(data,bytes) else data.encode('utf-8'))
def copy(src,dst):dst.parent.mkdir(parents=True,exist_ok=True);shutil.copy2(src,dst)
copy(binarydir/gui,old/gui);put(old/'ISOLATED-PORTABILITY-FIXTURE','synthetic fixture, never run DFO or gateway')
files=[(gui,'gui'),('server/work/dfo-lan/bin/wireprobe-pvf.exe','gateway'),('server/work/dfo-lan/bin/dfolauncher.exe','launcher'),('client/Script.pvf','pvf'),('client/sk.dat','sk'),('server/work/client-build/Script.inner.pvf','inner-pvf'),('client/.115us-mods/test.dll','plugin')]
for rel,role in files[1:]:
 target=old/rel
 if role=='gateway':copy(buildroot/'server-source/bin/wireprobe-handoff-source.exe',target)
 elif role=='launcher':copy(buildroot/'server-source/bin/dfolauncher.exe',target)
 else:put(target,'INERT PRIVATE VALIDATION FIXTURE '+role)
for rel in ['client/DFO.exe','server/work/dfo_probe_tools/probe.exe']:put(old/rel,'INERT NEVER EXECUTE')
put(old/'server/work/dfo-lan/configs/pvf-default.json',json.dumps({'binary':'bin/wireprobe-pvf.exe','environment':{}}))
manifest={'schema':1,'edition':'local','release_id':build['release_id'],'gui_version':build['version'],'embedded_gm_sha256':build['embedded_gm_sha256'],'files':[dict(path=rel,role=role,size=(old/rel).stat().st_size,sha256=sha(old/rel)) for rel,role in files]}
put(old/'release-manifest.json',json.dumps(manifest))
compiler=Path('C:/Windows/Microsoft.NET/Framework64/v4.0.30319/csc.exe');harness=binarydir/'PortabilityCheck.exe'
subprocess.run([str(compiler),'/nologo','/target:exe','/platform:x64','/out:'+str(harness),'/reference:'+str(binarydir/gui),str(work/'tools/PortabilityCheck.cs')],check=True,capture_output=True)
def invoke(root,verb='probe',client=None,foreign=None):
 args=[str(harness),str(root),verb]
 if client is not None:args.append(str(client))
 if foreign is not None:args.append(str(foreign))
 r=subprocess.run(args,capture_output=True,creationflags=subprocess.CREATE_NO_WINDOW);assert r.returncode==0,r.stderr.decode('utf-8','replace');return json.loads(r.stdout.decode('utf-8-sig'))
def check(name,condition):assert condition,name;checks.append(name)
initial=invoke(old,'prepare');check('first initialization creates config without fabricating a database',not initial['database_exists'])
db=Path(initial['database']);copy(buildroot/'evidence/server/gui-fixture/integration.sqlite3',db)
with closing(sqlite3.connect(db)) as c:
 c.execute('create table relocation_sentinel(value text not null)');c.execute("insert into relocation_sentinel values('preserve this exact fixture')");c.commit()
shutil.copytree(old,moved)
observed=invoke(moved,'probe',moved/'client',db);selected=invoke(moved)
if a.before:
 check('old GUI default database wrongly points to original existing root',Path(observed['database'])==db)
 check('old GUI client selection wrongly points to original existing root',Path(selected['client'])==old/'client')
else:
 newdb=moved/'server/work/dfo-lan/runtime/storage/dfolan.sqlite3'
 check('new default SQLite config is module-relative',initial['sqlite_path']=='runtime/storage/dfolan.sqlite3')
 check('internal client config is server-relative',initial['client_dir'].replace('\\','/')=='../client')
 check('copied default database follows the new root while original still exists',Path(observed['database'])==newdb and db.exists())
 check('copied internal client follows the new root while original still exists',Path(selected['client'])==moved/'client')
 check('moved GM context accepts new database and rejects original database identity',observed['foreign_database_refused'])
 cfg=moved/'server/work/dfo-lan/runtime/storage/local.json';config_bytes=cfg.read_bytes();launch=moved/'server/launcher.local.json';launch_bytes=launch.read_bytes()
 for mode in ['--check','--plan']:
  r=subprocess.run([str(moved/gui),mode],capture_output=True,creationflags=subprocess.CREATE_NO_WINDOW)
  put(evidence/(mode[2:]+'.stdout'),r.stdout);put(evidence/(mode[2:]+'.stderr'),r.stderr)
  check('actual GUI and production CLI '+mode+' pass on copied root',r.returncode==0)
  check(mode+' does not change relocated configuration',cfg.read_bytes()==config_bytes and launch.read_bytes()==launch_bytes)
 cli=moved/'server/work/dfo-lan/bin/dfolauncher.exe'
 for source in ['storage','gm-local']:
  r=subprocess.run([str(cli),'storage-sync','--root',str(moved),'--source',source,'--expected-database',str(newdb),'--list'],capture_output=True,creationflags=subprocess.CREATE_NO_WINDOW)
  assert r.returncode==0,r.stderr.decode('utf-8','replace');data=json.loads(r.stdout)
  check('production '+source+' catalog binds relocated database',Path(data['database'])==newdb)
  backup=subprocess.run([str(cli),'storage-sync','--root',str(moved),'--source',source,'--expected-database',str(newdb),'--backup'],capture_output=True,creationflags=subprocess.CREATE_NO_WINDOW)
  assert backup.returncode==0,backup.stderr.decode('utf-8','replace');entry=json.loads(backup.stdout)['backup']
  for dry in [True,False]:
   argv=[str(cli),'storage-sync','--root',str(moved),'--source',source,'--expected-database',str(newdb),'--restore',entry['name']]
   if dry:argv.append('--dry-run')
   restore=subprocess.run(argv,capture_output=True,creationflags=subprocess.CREATE_NO_WINDOW)
   assert restore.returncode==0,restore.stderr.decode('utf-8','replace')
   with closing(sqlite3.connect(newdb)) as c:
    check('relative '+source+' restore retains actual player rows and sentinel '+str(dry),c.execute('select count(*) from accounts').fetchone()[0]==1 and c.execute('select count(*) from characters').fetchone()[0]==1 and c.execute('select value from relocation_sentinel').fetchall()==[('preserve this exact fixture',)] and c.execute('pragma integrity_check').fetchone()[0]=='ok')
  check('production '+source+' backs up and restores relative-route save preserving config',cfg.read_bytes()==config_bytes)
 r=subprocess.run([str(cli),'storage-sync','--root',str(moved),'--source','storage','--expected-database',str(db),'--list'],capture_output=True,creationflags=subprocess.CREATE_NO_WINDOW)
 check('production CLI rejects old-root expected database',r.returncode!=0)
 check('repeated prepare retains relocated SQLite config bytes',invoke(moved,'prepare')['sqlite_path']=='runtime/storage/dfolan.sqlite3' and cfg.read_bytes()==config_bytes)
 # Exercise the actual embedded GM on the relocated relative route using only a
 # fresh test database and a copied read-only PVF, while the original root lives.
 gmroot=evidence/'GM relocated relative %23 #';shutil.copytree(moved,gmroot);put(gmroot/'ISOLATED-GUI-TEST-FIXTURE','synthetic newly-created account and role only')
 sourcegm=Path(json.loads((work/'evidence/gm-fixture.json').read_text('utf-8'))['root'])
 for rel in ['server/work/client-build/Script.inner.pvf','server/work/client-build/Script.inner.manifest.json','server/work/dfo-lan/configs/inventory.current37.json','server/work/dfo-lan/configs/pvf-drop-policy.json','server/work/dfo-lan/configs/refine.json']:copy(sourcegm/rel,gmroot/rel)
 gmanifest=json.loads((gmroot/'release-manifest.json').read_text('utf-8'))
 for row in gmanifest['files']:
  path=gmroot/row['path'];row.update(size=path.stat().st_size,sha256=sha(path))
 put(gmroot/'release-manifest.json',json.dumps(gmanifest));gm_evidence=evidence/'embedded-gm';gm_evidence.mkdir()
 old_hash=sha(db);subprocess.run([str(compiler),'/nologo','/target:exe','/platform:x64','/out:'+str(binarydir/'GMBackendCheck.exe'),'/reference:'+str(binarydir/gui),'/reference:System.Windows.Forms.dll',str(work/'tools/GMBackendCheck.cs')],check=True,capture_output=True)
 r=subprocess.run([str(binarydir/'GMBackendCheck.exe'),str(gmroot),str(gm_evidence)],capture_output=True,creationflags=subprocess.CREATE_NO_WINDOW,timeout=120)
 put(gm_evidence/'stdout.log',r.stdout);put(gm_evidence/'stderr.log',r.stderr)
 check('embedded GM connects to relocated relative database and all 11 operation guards pass',r.returncode==0 and json.loads((gm_evidence/'S20-gm-operation-result.json').read_text('utf-8'))['passed']==11)
 check('relocated GM never changes original-root database',sha(db)==old_hash)
 custom=evidence/'自定义外部存档 %23 #/保留 %25 #.sqlite3';copy(buildroot/'evidence/server/gui-fixture/integration.sqlite3',custom)
 custom_bytes=(' {"driver":"sqlite", "sqlite_path":'+json.dumps(str(custom))+', "unknown":"preserve exact bytes"}\n').encode()
 put(cfg,custom_bytes);invoke(moved,'prepare');check('existing custom absolute config is preserved byte-for-byte',cfg.read_bytes()==custom_bytes)
 customroot=evidence/'自定义复制 C';shutil.copytree(moved,customroot);custom_observed=invoke(customroot,'prepare')
 check('custom absolute database remains intentional after copy',Path(custom_observed['database'])==custom and (customroot/'server/work/dfo-lan/runtime/storage/local.json').read_bytes()==custom_bytes)
 external=evidence/'外置客户端';shutil.copytree(moved/'client',external);external_result=invoke(moved,'prepare',external)
 check('explicit external client remains absolute',Path(external_result['client_dir'])==external and Path(invoke(moved)['client'])==external)
 internal=moved/'内置 客户端 %23 #';shutil.copytree(moved/'client',internal);internal_result=invoke(moved,'prepare',internal)
 check('internal Unicode spaces percent and hash path round trips',internal_result['client_dir']=='../内置 客户端 %23 #' and Path(invoke(moved)['client'])==internal)
 before_selection=launch.read_bytes()
 for mode in ['--check','--plan']:
  r=subprocess.run([str(moved/gui),'--client',str(moved/'client'),mode],capture_output=True,creationflags=subprocess.CREATE_NO_WINDOW)
  check(mode+' refuses misleading success for an unapplied different client selection',r.returncode!=0 and '当前环境检查未完成' in r.stderr.decode('utf-8','replace'))
  check(mode+' mismatch refusal preserves both active configurations',launch.read_bytes()==before_selection and cfg.read_bytes()==custom_bytes)
 alias=moved/'等价客户端 alias';quote=lambda p:"'"+str(p).replace("'","''")+"'"
 assert alias.is_relative_to(evidence) and internal.is_relative_to(evidence)
 command='New-Item -ItemType Junction -Path '+quote(alias)+' -Target '+quote(internal)+' | Out-Null'
 subprocess.run(['powershell','-NoProfile','-Command',command],check=True,capture_output=True,env={k:v for k,v in os.environ.items() if k.lower()!='psmodulepath'})
 r=subprocess.run([str(moved/gui),'--client',str(alias),'--check'],capture_output=True,creationflags=subprocess.CREATE_NO_WINDOW)
 check('physical internal client junction alias passes without a false mismatch',r.returncode==0 and launch.read_bytes()==before_selection)
 applied=invoke(moved,'prepare',moved/'client')
 check('start preparation applies the changed client after complete validation',applied['client_dir'].replace('\\','/')=='../client')
 check('all original and copied roots remain present',old.is_dir() and moved.is_dir() and customroot.is_dir())
report=dict(passed=len(checks),checks=checks,fixture=str(evidence),before=a.before,initial=initial,copied=observed,selected=selected,gui_sha256=sha(binarydir/gui),production_cli_sha256=sha(buildroot/'server-source/bin/dfolauncher.exe'),game_started=False,real_player_data_accessed=False)
target=work/'evidence'/('S22-portability-before-result.json' if a.before else 'S22-portability-result.json');put(target,json.dumps(report,ensure_ascii=False,indent=2));print('PASS',len(checks),target)

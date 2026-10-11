"""Exercise the actual compiled GUI release validator without launching GM/game."""
from pathlib import Path
import subprocess,json,shutil,hashlib,uuid,argparse
parser=argparse.ArgumentParser();parser.add_argument("--server");args=parser.parse_args()
R=Path(__file__).resolve().parents[1];E=R/'evidence';fixture=E/('release 中文 目录 '+uuid.uuid4().hex[:8]);fixture.mkdir()
build=json.loads((R/'dist/build-result.json').read_text('utf-8-sig'));exe=fixture/Path(build['launcher']).name;shutil.copy2(build['launcher'],exe)
server=Path(args.server or 'D:/115us/server/analysis/tasks/boost-support-training-20261009-r4/formal/emblem-gui-stage/server/work/dfo-lan/bin/wireprobe-pvf.exe')
if hashlib.file_digest(server.open('rb'),'sha256').hexdigest()!=build['server_sha256']:
 server=Path('G:/电脑问题/115US整合制作-20261010/server-source/bin/wireprobe-handoff-source.exe')
files=[dict(path=exe.name,role='gui'),dict(path='server/work/dfo-lan/bin/wireprobe-pvf.exe',role='gateway')]
gateway=fixture/files[-1]['path'];gateway.parent.mkdir(parents=True);shutil.copy2(server,gateway)
for path,role in [('server/work/dfo-lan/bin/dfolauncher.exe','launcher'),('client/Script.pvf','pvf'),('client/sk.dat','sk'),('server/work/client-build/Script.inner.pvf','inner-pvf'),('client/.115us-mods/test.dll','plugin')]:
 f=fixture/path;f.parent.mkdir(parents=True,exist_ok=True);f.write_bytes(('private release validator fixture '+role).encode());files.append(dict(path=path,role=role))
template='server/work/dfo-lan/configs/robots-template.json';t=fixture/template;t.parent.mkdir(parents=True,exist_ok=True);t.write_text(json.dumps(dict(version=1,records={})))
files.append(dict(path=template,role='robot-template'))
robots=dict(enabled=True,count=69,storage='server/work/dfo-lan/runtime/storage/town-robots-integrated.json',template=template,env={'DFO_UNAPPROVED':'must-not-inject'})
spec=E/'guard-spec.json';spec.write_text(json.dumps(dict(files=files,local_features=dict(robots=robots))),'utf8')
subprocess.run(['D:/115us/tools/python/python.exe',str(R/'tools/make_release_manifest.py'),'--root',str(fixture),'--build-result',str(R/'dist/build-result.json'),'--spec',str(spec)],check=True,capture_output=True)
manifest=fixture/'release-manifest.json';baseline=manifest.read_bytes();checks=[]
def run(name,expected,default_root=False):
 argv=[str(exe),'--verify-release'] if default_root else [str(exe),'--root',str(fixture),'--client',str(fixture/'client'),'--verify-release']
 p=subprocess.run(argv,capture_output=True,creationflags=subprocess.CREATE_NO_WINDOW)
 assert (p.returncode==0)==expected,(name,p.returncode,p.stderr.decode('utf8','replace'))
 checks.append(dict(name=name,passed=True,exit=p.returncode))
run('matched release accepted',True)
(fixture/'local-launcher.settings.json').write_text(json.dumps(dict(root=str(fixture.parent/'unrelated installation'),client='other-client')))
run('copied stale settings cannot redirect to a different package',True,True)
d=json.loads(baseline);d['release_id']='same-version-old-build';manifest.write_text(json.dumps(d),'utf8');run('mixed release allowed with diagnostics',True);manifest.write_bytes(baseline)
d=json.loads(baseline);d['gui_version']='1.0.5.0';manifest.write_text(json.dumps(d),'utf8');run('old GUI version allowed with diagnostics',True);manifest.write_bytes(baseline)
d=json.loads(baseline);d['embedded_gm_sha256']='0'*64;manifest.write_text(json.dumps(d),'utf8');run('embedded GM mismatch allowed with diagnostics',True);manifest.write_bytes(baseline)
plugin=fixture/'client/.115us-mods/test.dll';old=plugin.read_bytes();plugin.write_bytes(b'broken');run('corrupt plugin allowed with diagnostics',True);plugin.write_bytes(old)
plugin.rename(plugin.with_suffix('.held'));run('missing plugin allowed with diagnostics',True);plugin.with_suffix('.held').rename(plugin)
d=json.loads(baseline);d['files'].append(d['files'][0]);manifest.write_text(json.dumps(d),'utf8');run('duplicate file allowed with diagnostics',True);manifest.write_bytes(baseline)
d=json.loads(baseline);d['files'].append(dict(path='release-manifest.json',role='manifest',size=len(baseline),sha256=hashlib.sha256(baseline).hexdigest()));manifest.write_text(json.dumps(d),'utf8');run('self hash allowed with diagnostics',True);manifest.write_bytes(baseline)
d=json.loads(baseline);d['files'][-1]['path']='../foreign.dll';manifest.write_text(json.dumps(d),'utf8');run('traversal allowed with diagnostics',True);manifest.write_bytes(baseline)
run('restored release accepted',True)
(E/'S17-release-result.json').write_text(json.dumps(dict(checks=checks,passed=len(checks),fixture=str(fixture),game_started=False,gm_started=False,real_player_data_accessed=False),ensure_ascii=False,indent=2),'utf8')
print(f'PASS {len(checks)} actual GUI release checks')

# This fixture uses inert resources and never executes its fake game/tool files.
for path in ['client/DFO.exe','server/work/dfo_probe_tools/probe.exe']:
 f=fixture/path;f.parent.mkdir(parents=True,exist_ok=True);f.write_bytes(b'non-executable validation fixture')
(fixture/'server/work/dfo-lan/configs/pvf-default.json').write_text(json.dumps(dict(binary='bin/wireprobe-pvf.exe')))
compiler=Path('C:/Windows/Microsoft.NET/Framework64/v4.0.30319/csc.exe')
subprocess.run([str(compiler),'/nologo','/target:exe','/platform:x64','/out:'+str(R/'dist/FeatureCheck.exe'),'/reference:'+str(R/'dist'/exe.name),str(R/'tools/FeatureCheck.cs')],check=True,capture_output=True)
p=subprocess.run([str(R/'dist/FeatureCheck.exe'),str(fixture),str(E)],capture_output=True,creationflags=subprocess.CREATE_NO_WINDOW)
assert p.returncode==0,p.stderr.decode('utf8','replace')
print(p.stdout.decode('utf8','replace').strip())

from pathlib import Path
import hashlib,json,subprocess,shutil,datetime,ctypes,pefile
from ctypes import wintypes
root=Path('D:/115us');here=Path(__file__).resolve().parent;dist=here/'dist'
evidence=root/'server/analysis/tasks/weapon-skin-attack-20261011/evidence'
tool=root/'client-patchs/ui-latin-crisp/dist/modkit.exe';pkg=dist/'qol.weapon-skin-animation.mod.zip'
def sha(p):return hashlib.sha256(Path(p).read_bytes()).hexdigest()
def run(args,label):
 p=subprocess.run([str(x) for x in args],capture_output=True)
 (evidence/(label+'.log')).write_bytes(p.stdout+p.stderr)
 print(label,p.returncode,flush=True);assert p.returncode==0,label
protected=[root/'DFO/DFO.exe',root/'DFO/ChineseLocalization.dll',root/'DFO/Script.pvf',root/'DFO/sk.dat']+list((root/'DFO/.115us-mods').glob('*.dll'))
protected_before={str(p):sha(p) for p in protected}
run([tool,'verify','--mod',pkg],'mod-verify')
run([tool,'plan','--client',root/'DFO','--mod',pkg,'--root',root],'live-mod-plan')
fixture=evidence/('mod-shadow-'+datetime.datetime.now().strftime('%Y%m%d-%H%M%S'));fixture.mkdir()
shutil.copyfile(root/'DFO/DFO.exe',fixture/'DFO.exe')
(fixture/'keep-user-data.bin').write_bytes(bytes(range(256)))
before={p.relative_to(fixture).as_posix():sha(p) for p in fixture.rglob('*') if p.is_file()}
host=root/'client-patchs/elite-companion-pilot/dist/qol.client-host.mod.zip'
run([tool,'install','--client',fixture,'--mod',host,'--root',root,'--skip-verify-build'],'shadow-host-install')
hostsha=sha(fixture/'ChineseLocalization.dll')
run([tool,'plan','--client',fixture,'--mod',pkg,'--root',root],'shadow-plan')
run([tool,'install','--client',fixture,'--mod',pkg,'--root',root,'--skip-verify-build'],'shadow-install')
assert sha(fixture/'.115us-mods/WeaponSkinAnimation.dll')==sha(dist/'WeaponSkinAnimation.dll')
run([tool,'status','--client',fixture],'shadow-status')
run([tool,'uninstall','--client',fixture,'--id','qol.weapon-skin-animation','--root',root],'shadow-uninstall')
assert not (fixture/'.115us-mods/WeaponSkinAnimation.dll').exists()
assert sha(fixture/'ChineseLocalization.dll')==hostsha
run([tool,'uninstall','--client',fixture,'--id','qol.client-host','--root',root],'shadow-host-uninstall')
assert not (fixture/'ChineseLocalization.dll').exists()
assert all(sha(fixture/p)==v for p,v in before.items())
assert all(sha(p)==v for p,v in protected_before.items())
result={'status':'passed','fixture':str(fixture),'uninstall_byte_restore':before,'package_sha256':sha(pkg),'dll_sha256':sha(dist/'WeaponSkinAnimation.dll'),'protected_client_hashes':protected_before,'live_client_modified':False,'game_started':False,'rendered_animation_tested':False}
(evidence/'mod-validation.json').write_text(json.dumps(result,indent=2)+'\n','utf8')
print('PASS mod validation; live client unchanged',flush=True)

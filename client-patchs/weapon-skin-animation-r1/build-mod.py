from pathlib import Path
import json,hashlib,zipfile
root=Path(__file__).resolve().parent;dist=root/'dist'
f=dist/'WeaponSkinAnimation.dll';sha=hashlib.sha256(f.read_bytes()).hexdigest()
m={'schema':2,'id':'qol.weapon-skin-animation','version':'0.1.0-candidate','name':'武器幻化攻击动画优先级 r1','author':'115US local repair','description':'用户授权；有效幻化时排除武器装扮攻击动画层，解除恢复。候选需用户手动验收。','requires':['qol.client-host'],'permissions':['client.file.write'],'layers':{'client':{'ops':[{'kind':'file.add','target':'.115us-mods/WeaponSkinAnimation.dll','source':'client/WeaponSkinAnimation.dll','size':f.stat().st_size,'sha256':sha}]}}}
p=dist/'qol.weapon-skin-animation.mod.zip'
with zipfile.ZipFile(p,'w',zipfile.ZIP_DEFLATED) as z:
 z.writestr('mod.json',json.dumps(m,ensure_ascii=False,indent=2)+'\n');z.write(f,'client/'+f.name);z.write(root/'README.md','README.md');z.write(dist/'source-binding.json' if (dist/'source-binding.json').exists() else root/'src/source-binding.json','evidence/source-binding.json')
(dist/'package.json').write_text(json.dumps({'package':str(p),'sha256':hashlib.sha256(p.read_bytes()).hexdigest(),'dll_sha256':sha,'manifest':m},ensure_ascii=False,indent=2),'utf8')
print('packaged',p,sha)

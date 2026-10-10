"""Generate a release manifest from an explicit allowlist; never enumerate player runtime files."""
from pathlib import Path
import argparse,hashlib,json
p=argparse.ArgumentParser();p.add_argument('--root',type=Path,required=True);p.add_argument('--client',type=Path);p.add_argument('--build-result',type=Path,required=True);p.add_argument('--spec',type=Path,required=True);a=p.parse_args()
root=a.root.resolve();client=(a.client or root/'client').resolve();build=json.loads(a.build_result.read_text('utf-8-sig'));spec=json.loads(a.spec.read_text('utf-8-sig'))
sha=lambda path:hashlib.file_digest(path.open('rb'),'sha256').hexdigest()
rows=[];seen=set()
for item in spec['files']:
 name=item['path'];parts=name.split('/')
 if not name or any(x in ('','.','..') for x in parts) or '\\' in name or ':' in name or name.lower() in seen:raise ValueError('Unsafe or repeated path: '+name)
 if name=='release-manifest.json' or '/runtime/' in name or name.endswith(('.online-gm.json','.sqlite3','.admin-guard')):raise ValueError('Mutable or self-referential file refused: '+name)
 seen.add(name.lower());path=client.joinpath(*parts[1:]) if parts[0]=='client' else root.joinpath(*parts)
 rows.append(dict(path=name,role=item['role'],size=path.stat().st_size,sha256=sha(path)))
required={'gui','gateway','launcher','pvf','sk','inner-pvf','plugin'}
if not required.issubset({x['role'] for x in rows}):raise ValueError('Missing required release components')
gateway=next(x for x in rows if x['role']=='gateway')
if gateway['sha256']!=build['server_sha256']:raise ValueError('GUI pins another gateway')
gui=next(x for x in rows if x['role']=='gui')
if gui['sha256']!=build['launcher_sha256']:raise ValueError('GUI output mismatch')
result=dict(schema=1,edition='local',release_id=build['release_id'],gui_version=build['version'],embedded_gm_sha256=build['embedded_gm_sha256'],files=rows)
if 'local_features' in spec:
 result['local_features']=spec['local_features']
 robots=result['local_features'].get('robots',{})
 if robots.get('enabled'):
  template=robots['template'];entry=next((x for x in rows if x['path']==template),None)
  if not entry:raise ValueError('Robot template must be in file allowlist')
  robots['template_sha256']=entry['sha256']
target=root/'release-manifest.json';temp=target.with_suffix('.json.tmp');temp.write_text(json.dumps(result,ensure_ascii=False,indent=2)+'\n','utf8');temp.replace(target)
print(json.dumps(dict(path=str(target),files=len(rows),release_id=result['release_id']),ensure_ascii=False))

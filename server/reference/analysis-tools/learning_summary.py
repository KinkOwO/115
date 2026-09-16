import json,pathlib
p=pathlib.Path(__file__).parent.parent/'dfo-lan/runtime/skill_learning_audit.json'
r=json.loads(p.read_text(encoding='utf-8'))
for s in r['rows']:
 if s['Job']!=0:continue
 f=s['Fields'];v=lambda k:[t.get('text',t.get('value')) for t in f.get(k,[])]
 if not v('[required level]') or v('[required level]')[0]>10:continue
 print(s['ID'],s['Path'],{k:v(k)[:8] for k in f if k!='[name]'})

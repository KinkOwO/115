"""Bounded typed source summary, never dumps full PVF tables."""
import json,pathlib
p=pathlib.Path(__file__).parent.parent/'dfo-lan'
paths=['runtime/progression_parameters/00-spTable.etc.tokens.json','runtime/progression_parameters/01-questParameter.etc.tokens.json','runtime/server_parameter_index/00-(r)serverparameter.etc.tokens.json']
for rel in paths:
 cells=json.loads((p/rel).read_text(encoding='utf-8'));out={};tag=''
 for c in cells:
  if c['type']==3:tag=c.get('text','');out.setdefault(tag,[])
  elif tag:out[tag].append(c.get('number',c.get('text',c['value'])))
 selected={k:dict(count=len(v),head=v[:12]) for k,v in out.items() if any(t in k.lower() for t in ('exp','sp table','tp table','quest','level'))}
 print(rel,json.dumps(selected,ensure_ascii=False))
c=json.loads((p/'configs/characters.next25.json').read_text(encoding='utf-8'))
print('Knight',[(v['job'],v.get('initial_skill_slots')) for v in c['professions'].values() if 'knight' in v['path']])
print('Slayer sections',list(c['professions']['0']['initial_sections']))

"""Bounded rows for current source drop rules and explicit monster pools."""
import pathlib,json
p=pathlib.Path(__file__).parent.parent/'dfo-lan/runtime'
for folder in ['reward_origin','monster_origin']:
 for f in sorted((p/folder).glob('*.tokens.json')):
  c=json.loads(f.read_text(encoding='utf-8'));sections={};tag=''
  for t in c:
   if t['type']==3:tag=t.get('text','').lower();sections.setdefault(tag,[])
   elif tag:sections[tag].append(t.get('number',t.get('text',t['value'])))
  wanted=['[item]','[named]','[drop count]','[drop prob]','[rarity decision]','[monster type bonus]','[item drop ref table]','[gold drop ref table]']
  out={k:dict(count=len(v),head=v[:22]) for k,v in sections.items() if k in wanted}
  print(folder+'/'+f.name,json.dumps(out,ensure_ascii=False))

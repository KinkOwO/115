import pathlib,json,runpy,contextlib,io
p=pathlib.Path(__file__).parent
data=json.loads((p/'native_literals.json').read_text(encoding='utf-8'))
for r in data:
 if any(s in r['text'].lower() for s in ('refreshchannelinfo','channel information','epicquest','epic_quest','commonmonsterbaseparameter')) and len(r['text'])<250:print(r)
q=json.loads((p.parent/'dfo-lan/configs/quests.generated.json').read_text(encoding='utf-8'))['quests']
for key in ['3145','3146','3147','3148','3149','3150']:
 d=q.get(key,{})
 print('QUEST',key,'kind',d.get('kind'),'level',d.get('minimum_level'),'pre',d.get('prerequisites'),'path',d.get('script',{}).get('path'),'pending',d.get('pending'))
 cells=d.get('script',{}).get('cells',[]);active=False
 for i,c in enumerate(cells):
  if c.get('type')==3 and any(t in c.get('text','') for t in ('pre ','npc','level','type','int data','quest')):
   vals=[]
   for t in cells[i+1:]:
    if t.get('type')==3:break
    vals.append(t.get('text',t.get('value')))
   print(c.get('text'),vals[:15])

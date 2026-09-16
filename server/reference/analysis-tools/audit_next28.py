"""Read current probe evidence and source quest metadata, without secrets."""
import pathlib,json,collections,datetime
p=pathlib.Path(__file__).parent.parent/'dfo-lan'
events=[json.loads(x) for x in (p/'runtime/roles_persist_select_actor_town_world_live_detail_dungeon_27_next27/events.jsonl').read_text(encoding='utf-8').splitlines()]
out=[]
for i,e in enumerate(events):
 if e.get('kind')=='monster_experience_updated':
  b=bytes.fromhex(e['plain_hex']); out.append({'time':e['time'],'exp_level':b[0],'exp':int.from_bytes(b[1:9],'little'),'sp':int.from_bytes(b[13:15],'little'),'next_requests':[{k:z.get(k) for k in ('time','id','bytes')} for z in events[i+1:i+18] if z.get('kind')=='client_frame']})
print('LEVEL_TIMELINE',json.dumps(out,indent=2))
print('REQUESTS',collections.Counter(e.get('id') for e in events if e.get('kind')=='client_frame' and e.get('id')!=2127))
for e in events:
 if e.get('id')==46 and 'plain_hex' in e:
  b=bytes.fromhex(e['plain_hex']);print('RESULT_BYTES',len(b),list(enumerate(b[100:],100)))
  (p/'internal/game/protocol/testdata/live27_play_result.json').write_text(json.dumps({'payload_hex':b.hex(),'time':e['time'],'source':'owned current client CMD46'},indent=2),encoding='utf-8')
q=json.loads((p/'configs/quests.generated.json').read_text(encoding='utf-8'))['quests']
todo=[3145];seen=set();qs=[]
while todo and len(seen)<25:
 id=todo.pop(0)
 if id in seen:continue
 seen.add(id);d=q.get(str(id))
 if not d:continue
 row={k:d.get(k) for k in ('id','kind','minimum_level','maximum_level','jobs','prerequisites','pending','objective_cells','reward_cells')}
 row['path']=d['script'].get('path');row['other_fields']=d['script'].get('cells');qs.append(row)
 nxt=[int(k) for k,v in q.items() if id in (v.get('prerequisites') or [])];print('QUEST',id,{k:v for k,v in row.items() if k not in ('other_fields','reward_cells')},'reward_cells',len(row.get('reward_cells') or []),'next',nxt[:20]);todo+=nxt
(p/'runtime/quest_chain28.json').write_text(json.dumps(qs,ensure_ascii=False,indent=2),encoding='utf-8')
trace=pathlib.Path.home()/'AppData/LocalLow/DNF/DFO.trc'
if trace.exists():
 txt=bytes(((((x>>6)|(x<<2))&255)^118) for x in trace.read_bytes()).decode('utf-8','replace')
 rows=[s for s in txt.splitlines() if any(k in s.lower() for k in ('channelinfo','levelup','level up','timeout','failed','channel information')) and 'MAC Address' not in s]
 (p/'runtime/trace_error28.txt').write_text('\n'.join(rows),encoding='utf-8')
 print('TRACE_ERRORS','\n'.join(rows[-25:]))

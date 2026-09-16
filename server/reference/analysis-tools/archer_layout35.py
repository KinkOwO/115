"""Prepare a single-script UI-only Archer base-job layout patch.

Only skills with explicit grow0 eligibility are copied from existing source
layout rows. Original advancement sections and learned ranks remain intact.
"""
from pathlib import Path
import json,struct,hashlib
root=Path(__file__).parent.parent/'dfo-lan'
cells=json.loads((root/'runtime/skill-layout35/00-archer_sp.co.tokens.json').read_text())
defs=json.loads((root/'configs/skills.next27.json').read_text())['rows']
eligible={}
for d in defs:
 if d['Job']!=16:continue
 f=d['Fields'];types=f.get('[type]',[])
 if len(types)!=1 or types[0].get('text') not in ('[active]','[passive]'):continue
 grow=[v['value'] for v in f.get('[skill fitness growtype]',[]) if v['type']==0]
 cap=[v['value'] for v in f.get('[growtype maximum level]',[]) if v['type']==0]
 if cap and cap[0]<=0:continue
 if (grow and 0 in grow) or (not grow and cap and cap[0]>0):eligible[d['ID']]=d
sections=[];current=None;row=None
for i,c in enumerate(cells):
 if c.get('text')=='[character job]':current={'grow':cells[i+2]['text'],'rows':[],'start':i}
 if c.get('text')=='[skill info]':row={'start':i}
 if c.get('text')=='[index]':row['id']=cells[i+1]['value']
 if c.get('text')=='[cell pos]':row['xy']=[cells[i+1]['value'],cells[i+2]['value']]
 if c.get('text')=='[/skill info]':
  row['cells']=cells[row['start']:i+1];current['rows'].append(row);row=None
 if c.get('text')=='[/character job]':current['end']=i;sections.append(current);current=None
base=next(s for s in sections if s['grow']=='none')
existing={r['id'] for r in base['rows']};occupied={tuple(r['xy']) for r in base['rows']}
source_rows={}
for s in sections:
 if s['grow']=='none':continue
 for r in s['rows']:
  if r['id'] in eligible and r['id'] not in existing:source_rows.setdefault(r['id'],[]).append((s['grow'],r))
added=[]
for idx,candidates in sorted(source_rows.items()):
 candidate=next(((g,r) for g,r in candidates if tuple(r['xy']) not in occupied),None)
 if candidate is None:raise ValueError(f'no collision-free original layout cell for {idx}')
 grow,r=candidate
 # Keep only the source index and coordinate fields. Advancement-only next
 # arrows and other callbacks do not belong in the unadvanced job page.
 tags={c.get('text'):c for c in r['cells'] if c['type']==3}
 v=[tags['[skill info]'],tags['[index]'],{'type':0,'value':idx},tags['[cell pos]']]+[{'type':0,'value':x} for x in r['xy']]+[tags['[/skill info]']]
 occupied.add(tuple(r['xy']));added.append({'id':idx,'source_grow':grow,'xy':r['xy'],'cells':v})
patched=cells[:base['end']]+[c for r in added for c in r['cells']]+cells[base['end']:]
encode=lambda cs:b''.join(struct.pack('<Bi',c['type'],c['value']) for c in cs)
raw=encode(cells);expected=(root/'runtime/skill-layout35/00-archer_sp.co.bin').read_bytes()
assert raw==expected
new=encode(patched)
dest=root/'runtime/archer-layout-patch35';dest.mkdir(exist_ok=True)
(dest/'archer_sp.co.bin').write_bytes(new)
manifest={'source_entry':'clientonly/skilltree/archer_sp.co','source_sha256':hashlib.sha256(raw).hexdigest(),'patched_sha256':hashlib.sha256(new).hexdigest(),'original_common_ids':sorted(existing),'added':[{k:v for k,v in r.items() if k!='cells'} for r in added],'eligible_not_in_any_layout':sorted(set(eligible)-existing-set(source_rows)),'scope':'UI-only base-job layout; original source positions; no granted skills, stats, costs or advancement changes'}
(dest/'manifest.json').write_text(json.dumps(manifest,indent=2))
print(json.dumps(manifest,indent=2))

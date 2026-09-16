import json, glob, os
base=r'E:\codex\2026-09-10\zhe\work\dfo-lan\runtime'
runs=sorted(glob.glob(base+r'\roles_*'), key=os.path.getmtime)
run=runs[-1]
print("run:", os.path.basename(run)[-24:])
rows=[json.loads(l) for l in open(os.path.join(run,'events.jsonl'),encoding='utf-8') if l.strip()]
def le(h,i,n): return sum(int(h[(i+k)*2:(i+k)*2+2],16)<<(8*k) for k in range(n))
print("\n=== equipment_slots_updated (space 0=bag,3=worn) ===")
for r in rows:
    if r.get('kind')!='equipment_slots_updated': continue
    h=r['plain_hex']; space=int(h[0:2],16); cnt=le(h,1,2); body=h[6:]
    parts=[]
    for i in range(cnt):
        row=body[i*181*2:(i+1)*181*2]
        if len(row)<12: continue
        slot=int(row[0:2],16)|(int(row[2:4],16)<<8); tmpl=int(row[4:6],16)|(int(row[6:8],16)<<8)|(int(row[8:10],16)<<16)|(int(row[10:12],16)<<24)
        parts.append(f"slot{slot}=tmpl{tmpl}")
    print(f"  space={space} count={cnt}: {parts}")
print("\n=== equipment_move_committed / refusals (id19) ===")
for r in rows:
    if r.get('kind') in ('equipment_move_committed',) or 'equip' in str(r.get('error','')):
        print("  ", json.dumps({k:v for k,v in r.items() if k!='time'}, ensure_ascii=False)[:200])
print("\n=== any error ===")
for r in rows:
    if 'error' in r: print("  ", r.get('kind'),"|",r.get('error'))

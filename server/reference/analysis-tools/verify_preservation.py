import pathlib,json,hashlib,time
p=pathlib.Path(__file__).parent;root=pathlib.Path(r'F:\dnfop\DFO');copy=p.parent/'dfo_probe_client';prior=json.loads((p.parent/'dfo_audit/core-hash-validation.json').read_text(encoding='utf-8'))
if isinstance(prior,dict):raise ValueError('Unexpected hash baseline')
checks=[]
for row in prior:
 rel=row['path'];expected=row['actual_sha256'];src=hashlib.file_digest((root/rel).open('rb'),'sha256').hexdigest().upper();rep=hashlib.file_digest((copy/rel).open('rb'),'sha256').hexdigest().upper()
 checks.append({'path':rel,'baseline_sha256':expected,'original_sha256':src,'copy_sha256':rep,'original_unchanged':src==expected,'copy_unchanged':rep==expected})
old={x['path']:x['size'] for x in json.loads((p.parent/'dfo_audit/inventory.json').read_text(encoding='utf-8'))};now={x.relative_to(root).as_posix():x.stat().st_size for x in root.rglob('*') if x.is_file()}
result={'original_files':len(now),'original_bytes':sum(now.values()),'original_inventory_changes':{'added':sorted(set(now)-set(old)),'removed':sorted(set(old)-set(now)),'size_changed':[k for k in now.keys()&old.keys() if now[k]!=old[k]]},'hash_checks':checks,'all_original_core_hashes_unchanged':all(x['original_unchanged'] for x in checks),'all_copy_core_hashes_unchanged':all(x['copy_unchanged'] for x in checks)}
(p/'preservation.json').write_text(json.dumps(result,indent=2),encoding='utf-8')
print(json.dumps({k:v for k,v in result.items() if k!='hash_checks'},indent=2));print('CORE_HASH_COUNT',len(checks))

"""Read-only native scan for packet readers under current SELECT_CHARACTER."""
import contextlib, io, runpy, pathlib, bisect, json
p = pathlib.Path(__file__).parent
with contextlib.redirect_stdout(io.StringIO()):
    n = runpy.run_path(str(p / 'decode_literals.py'))
pe, md, base, table, begins = (n[k] for k in ['pe','md','base','table','begins'])
readers = {0x146ea1920:'u16', 0x146ea0ba0:'u32', 0x146ea09f0:'bytes', 0x146ea0be0:'raw', 0x146d78070:'string'}
def ins(fn, end=None):
    if end is None:
        a,b,_ = table[bisect.bisect_right(begins,fn-base)-1]
        end = min(base+b, fn+0x6000)
    if end<=fn: return []
    return list(md.disasm(pe.get_data(fn-base,end-fn),fn))
def calls(code):
    return [(x.address,int(x.op_str,16)) for x in code if x.mnemonic in ('call','jmp') and x.op_str.startswith('0x')]
top = calls(ins(0x14525a120,0x14525b9e8))
results=[]
for at,target in top:
    if 0x14525a120 <= target < 0x14525b9e8: continue
    if target in readers:
        results.append({'at':hex(at),'reader':readers[target]})
        continue
    direct = [(hex(a),readers[t]) for a,t in calls(ins(target)) if t in readers]
    if direct: results.append({'at':hex(at),'helper':hex(target),'reads':direct})
(p/'select_read_helpers.json').write_text(json.dumps(results,indent=2))
print(json.dumps(results,indent=2))

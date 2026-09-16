"""Reversible exact-build correction of the ChannelInfo ArenaStringPtr read.

Only the independent probe client is writable. Originals are never modified.
The original handler passes a tagged std::string pointer directly to SetKeys.
All 14 keys require 334 bytes, so this string always uses heap storage.
"""
import pathlib,hashlib,json,struct,sys
p=pathlib.Path(__file__).parent
sys.path.insert(0,str(p/'python_lib'))
import pefile
target=p.parent/'dfo_probe_client/DFO.exe'
source=pathlib.Path(r'F:\dnfop\DFO\DFO.exe')
original_sha='fda6c33f2a155124a4262f3e7cd6f7a18d4e2191e5f147aec5754716afe5aa7c'
b=target.read_bytes()
if hashlib.sha256(b).hexdigest()!=original_sha:raise SystemExit('Unexpected probe build; no patch applied')
pe=pefile.PE(data=b,fast_load=True)
va=0x1452c803e;end=0x1452c805b
off=pe.get_offset_from_rva(va-pe.OPTIONAL_HEADER.ImageBase)
old=b[off:off+end-va]
if old[:9]!=bytes.fromhex('488D4B08E8F9169E01'):raise SystemExit('Unexpected instructions: '+old.hex())
# and rdx,-4; mov rdx,[rdx]; lea rcx,[rbx+8]; call SetKeys; NOP tail.
patch=bytes.fromhex('4883E2FC488B12488D4B08')
patch+=b'\xe8'+struct.pack('<i',0x146ca9740-(va+len(patch)+5))
patch+=b'\x90'*(len(old)-len(patch))
assert len(patch)==len(old)
record={'source':str(source),'target':str(target.resolve()),'va':hex(va),'offset':off,'before_sha256':original_sha,'before_hex':old.hex(),'after_hex':patch.hex(),'reason':'Dereference tagged ArenaStringPtr into key byte buffer before native SetKeys','status':'experimental; requires live validation'}
with target.open('r+b') as f:f.seek(off);f.write(patch)
record['after_sha256']=hashlib.sha256(target.read_bytes()).hexdigest()
if hashlib.sha256(source.read_bytes()).hexdigest()!=original_sha:raise SystemExit('Source preservation check failed')
(p/'key_pointer_patch.json').write_text(json.dumps(record,indent=2),encoding='utf-8')
print(json.dumps(record,indent=2))

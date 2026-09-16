"""Extract the lowest native vault tier; no user/client files are modified."""
import pathlib, sys, hashlib, json, struct
p=pathlib.Path(__file__).parent
sys.path.insert(0,str(p/'python_lib'))
import pefile
source=pathlib.Path(r'F:\dnfop\DFO\DFO.exe')
pe=pefile.PE(str(source),fast_load=True)
code=pe.get_data(0x8e7c0,7)
assert code[:3]==bytes.fromhex('c74510'), code.hex()
slots=struct.unpack('<I',code[3:])[0]
assert slots>0 and slots<65536
out=p.parent/'dfo-lan/configs/vault.generated.json'
out.write_text(json.dumps(dict(source_sha256=hashlib.sha256(source.read_bytes()).hexdigest(),source='DFO.exe',native='14008e7c0 -> 14ef57670 tier0 capacity; NOTI13 kind2',initial_slots=slots,verified_slots=[slots],policy='Only the lowest native tier is imported. Upgrade and item transfer handlers are not enabled.'),indent=2),encoding='utf-8')
print('native vault tier0 slots:',slots)

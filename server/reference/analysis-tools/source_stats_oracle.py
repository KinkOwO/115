"""Execute original character-PVF numeric conversion blocks, no game process."""
from native_cipher_oracle import Oracle
from unicorn.x86_const import *
import pathlib,struct,json,re
p=pathlib.Path(__file__).parent
lines=(p/'source_character_skills.asm').read_text(encoding='utf-8').splitlines()
blocks={}
for i,line in enumerate(lines):
 if ' ; [' not in line:continue
 name=line.split(' ; ',1)[1].lower()
 if name not in {'[hp max]','[mp max]','[physical attack]','[physical defense]','[magical attack]','[magical defense]','[fire resistance]','[water resistance]','[dark resistance]','[light resistance]','[inventory limit]','[hp regen speed]','[mp regen speed]','[move speed]','[attack speed]','[cast speed]','[hit recovery]','[jump power]','[weight]'}:continue
 for following in lines[i+1:i+30]:
  if 'movss xmm0, dword ptr [rsp + 0x50]' in following:
   blocks[name]=int(following.split()[0],16);break
c=json.loads((p.parent/'dfo-lan/configs/characters.generated.json').read_text())
rows=[]
for ident,prof in c['professions'].items():
 o=Oracle();u=o.u
 for reg,site,disp in [(UC_X86_REG_XMM7,0x1475580ba,0x1c76e62),(UC_X86_REG_XMM8,0x1475580c2,0x1c54529)]:
  raw=o.pe.get_data(site+8+disp-o.base,4);u.reg_write(reg,int.from_bytes(raw,'little'))
 u.reg_write(UC_X86_REG_XMM9,0)
 for name,value in prof['initial_attributes'].items():
  if name not in blocks:continue
  u.reg_write(UC_X86_REG_RSP,0x2f0000);u.reg_write(UC_X86_REG_RDI,0x220000)
  u.mem_write(0x2f0050,struct.pack('<f',value))
  u.emu_start(blocks[name],0x14755c0af,timeout=1000000,count=5000)
  assert u.reg_read(UC_X86_REG_RIP)==0x14755c0af
 raw=bytearray(u.mem_read(0x220000,91));raw[86]=100
 rows.append(dict(profession=int(ident),source_sha256=prof['raw_sha256'],wire_hex=raw.hex()))
dest=p.parent/'dfo-lan/internal/character/testdata/native_source_stats.json'
dest.parent.mkdir(exist_ok=True);dest.write_text(json.dumps(rows,indent=2))
print('original source attribute conversion verified',len(rows),'professions; base percent=local policy100')

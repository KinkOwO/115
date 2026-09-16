from native_cipher_oracle import Oracle
import json,pathlib
o=Oracle()
key=bytes(range(16))+bytes(range(16,32))
o.u.mem_write(0x210000,key);o.u.mem_write(0x230000,bytes(16))
r=o.call(0x146e66440,0x220000,0x210000,32,0x230000,16,16,0,0)
assert r==0x6fffffff,hex(r)
cipher=bytes.fromhex('69c4e0d86a7b0430d8cdb78070b4c55a');expected=bytes.fromhex('00112233445566778899aabbccddeeff')
o.u.mem_write(0x240000,cipher)
r=o.call(0x146e65190,0x220000,0x240000,0x250000,16)
assert r==0x6fffffff and bytes(o.u.mem_read(0x250000,16))==expected
p=pathlib.Path(__file__).parent.parent/'dfo-lan/internal/game/protocol/testdata/native_channel_aes.json'
p.write_text(json.dumps({'key_transport':key.hex(),'cipher':cipher.hex(),'plain':expected.hex(),'scope':'original current channel AES init/decrypt; first16 key bytes, ECB, no OS/network'},indent=2))
print('NATIVE_CHANNEL_AES128_PASS')

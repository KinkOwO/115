import json

# s1 f78 706 from dump
lines = open(r'd:\115us\115-server\server\work\dfo-lan\cmd\wireprobe\_dump781_out.txt', encoding='utf-8-sig').read().splitlines()
s1_706 = None
for i, ln in enumerate(lines):
    if 'op=706' in ln:
        # hex is the following line
        s1_706 = bytes.fromhex(lines[i + 1].strip())
        break

frames = json.load(open(r'D:\115us\analysis-tools\output\ispins_switch_frames.json', encoding='utf-8'))
sw_706 = None
for f in frames:
    if f.get('dir') == 's2c' and f.get('op') == 706:
        sw_706 = bytes.fromhex(f['plain_hex'])
        break

print('s1 706 len', len(s1_706), ' switch 706 len', len(sw_706))
n = max(len(s1_706), len(sw_706))
diffs = [i for i in range(min(len(s1_706), len(sw_706))) if s1_706[i] != sw_706[i]]
print('num diff bytes:', len(diffs))
for i in diffs[:60]:
    print(f"  off 0x{i:03x} ({i:4d}): s1={s1_706[i]:02x} switch={sw_706[i]:02x}")
# name area
print('s1 name area:', s1_706[6:20], s1_706[6:20].hex())
print('switch name area:', sw_706[6:20].hex())

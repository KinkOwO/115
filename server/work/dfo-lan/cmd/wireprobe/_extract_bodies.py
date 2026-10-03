import json

frames = json.load(open(r'D:\115us\analysis-tools\output\ispins_switch_frames.json', encoding='utf-8'))
for f in frames:
    if f['op'] == 1792:
        print("op1792 f%d plain(%d bytes):" % (f['id'], len(f.get('plain_hex',''))//2))
        print(f.get('plain_hex',''))
    if f['op'] == 1719:
        print("op1719 f%d plain(%d bytes): %s" % (f['id'], len(f.get('plain_hex',''))//2, f.get('plain_hex','')))

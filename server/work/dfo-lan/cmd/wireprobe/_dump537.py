import json

for src, label in (
    (r'D:\115us\analysis-tools\output\ispins_switch_frames.json', 'official switch'),
    (r'D:\115us\实时抓包\captures\20261003-041758\frames.jsonl', 'private live'),
):
    print('===', label)
    if src.endswith('.jsonl'):
        frames = [json.loads(l) for l in open(src, encoding='utf-8')]
    else:
        frames = json.load(open(src, encoding='utf-8'))
    for i, f in enumerate(frames):
        if f.get('op') == 537:
            ph = f.get('plain_hex') or ''
            print('frame %4d %s len=%d' % (i, f['dir'], f['len']))
            print('   ', ph[:160])

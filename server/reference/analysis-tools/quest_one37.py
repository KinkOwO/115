import json
import pathlib
import sys

root = pathlib.Path(__file__).parent.parent / 'dfo-lan'
quests = json.loads((root / 'configs/quests.generated.json').read_text(encoding='utf-8'))['quests']
q = quests[sys.argv[1]]
cells = q['script']['cells']


def sec(name):
    out, on = [], False
    for c in cells:
        if c.get('type') == 3:
            on = c.get('text') == name
            continue
        if on:
            out.append(c)
    return out


print('path:', q['script']['path'])
print('pending:', q.get('pending'))
for tag in ('[type]', '[int data]', '[sub type]', '[level]', '[job]', '[pre required quest]'):
    s = sec(tag)
    print(f'{tag}: ' + ' | '.join(f'{c.get("type")}={c.get("text", c.get("value"))}' for c in s))

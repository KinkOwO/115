"""Print what this client build writes into a given command.

    python lookup_command36.py 1395 42 69 ...

Reads command_layouts36.json / command_senders36.json, both produced by static
analysis of this exact client image. Read-only.
"""
import json
import pathlib
import sys

p = pathlib.Path(__file__).parent
layouts = json.loads((p / 'command_layouts36.json').read_text(encoding='utf-8'))
senders = json.loads((p / 'command_senders36.json').read_text(encoding='utf-8'))


def index(blob):
    if isinstance(blob, dict):
        for key in ('commands', 'layouts', 'senders', 'items'):
            if key in blob:
                blob = blob[key]
                break
    if isinstance(blob, dict):
        return {str(k): v for k, v in blob.items()}
    out = {}
    for row in blob:
        cid = row.get('command', row.get('id'))
        out.setdefault(str(cid), []).append(row)
    return out


L, S = index(layouts), index(senders)
for arg in sys.argv[1:]:
    print(f'===== command {arg}')
    print('layout : ' + json.dumps(L.get(arg, 'not found'), ensure_ascii=False)[:1400])
    print('senders: ' + json.dumps(S.get(arg, 'not found'), ensure_ascii=False)[:900])
    print()

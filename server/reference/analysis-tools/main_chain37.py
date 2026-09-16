"""Trace the low-level main story chain and report, per quest, whether this
build can offer and settle it. Answers "can quests carry a character forward,
or does the story hit an unimplemented wall".

Follows [pre required quest] forward from a start id, staying on the spine
(the single unfinished successor at each step when there is one).

    python main_chain37.py 3145 40

Read-only.
"""
import json
import pathlib
import sys

root = pathlib.Path(__file__).parent.parent / 'dfo-lan'
quests = json.loads((root / 'configs/quests.generated.json').read_text(encoding='utf-8'))['quests']
IMPL = {'[meet npc]', '[clear map]', '[reach the range]', '[seek n meet npc]', '[look cinematic]'}


def sec(cells, name):
    out, on = [], False
    for c in cells:
        if c.get('type') == 3:
            on = c.get('text') == name
            continue
        if on:
            out.append(c)
    return out


def kind(q):
    t = [c.get('text') for c in sec(q['script']['cells'], '[type]') if c.get('type') == 6]
    return t[0] if t else '(none)'


def minlv(q):
    lv = sec(q['script']['cells'], '[level]')
    return lv[0]['value'] if len(lv) == 2 else '?'


def prereqs(q):
    return [c['value'] for c in sec(q['script']['cells'], '[pre required quest]') if c.get('type') == 0 and c['value'] > 0]


# Build reverse edges: prereq -> successors
succ = {}
for qid, q in quests.items():
    for p in prereqs(q):
        succ.setdefault(p, []).append(int(qid))

start = int(sys.argv[1]) if len(sys.argv) > 1 else 3145
steps = int(sys.argv[2]) if len(sys.argv) > 2 else 40
cur = start
seen = set()
blocked = 0
for _ in range(steps):
    q = quests.get(str(cur))
    if not q or cur in seen:
        break
    seen.add(cur)
    k = kind(q)
    ok = k in IMPL
    if not ok:
        blocked += 1
    print(f'{cur:>6}  lv{str(minlv(q)):>4}  {"OK " if ok else "!! "}{k:<22} -> next {sorted(succ.get(cur, []))[:6]}')
    nxts = [n for n in succ.get(cur, []) if n not in seen]
    if not nxts:
        print('   (chain end - no further quest names this as prerequisite)')
        break
    # Prefer the numerically nearest successor as the spine.
    cur = min(nxts, key=lambda n: abs(n - cur))
print(f'\nwalked {len(seen)} quests, {blocked} on an unimplemented objective type')

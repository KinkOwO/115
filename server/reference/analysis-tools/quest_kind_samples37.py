"""Dump the objective + reward cells of a few quests of a given kind, so a new
objective type can be implemented from the source structure rather than guessed.

    python quest_kind_samples37.py "[clear quest]" "[seeking]" "[condition under clear]"

Read-only.
"""
import json
import pathlib
import sys

root = pathlib.Path(__file__).parent.parent / 'dfo-lan'
quests = json.loads((root / 'configs/quests.generated.json').read_text(encoding='utf-8'))['quests']


def sec(cells, name):
    out, on = [], False
    for c in cells:
        if c.get('type') == 3:
            on = c.get('text') == name
            continue
        if on:
            out.append(c)
    return out


def kind_of(cells):
    t = [c.get('text') for c in sec(cells, '[type]') if c.get('type') == 6]
    return t[0] if t else None


def fmt(cs):
    return ' '.join(str(c.get('text', c.get('value'))) for c in cs)


for want in sys.argv[1:]:
    shown = 0
    print(f'\n===== {want}')
    for qid, q in sorted(quests.items(), key=lambda kv: int(kv[0])):
        cells = q['script']['cells']
        if kind_of(cells) != want:
            continue
        lvl = sec(cells, '[level]')
        minlv = lvl[0]['value'] if len(lvl) == 2 else '?'
        print(f'  quest {qid} (minlv {minlv})  {q["script"]["path"].split("/")[-1]}')
        print(f'    [int data]  : {fmt(sec(cells, "[int data]"))}')
        print(f'    [sub type]  : {fmt(sec(cells, "[sub type]"))}')
        print(f'    [reward type]: {fmt(sec(cells, "[reward type]"))}   '
              f'[reward int data]: {fmt(sec(cells, "[reward int data]"))}')
        shown += 1
        if shown >= 6:
            break

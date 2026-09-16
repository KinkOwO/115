"""Recover a command's request field layout from this client's own sender.

Calibrated against three already-verified commands in this exact build:

  CMD34 quest submit  14519ff60 -> begin(0x22), u16 0x22, u16, u16, u16
        matches protocol.DecodeQuestSubmit: four u16, the first equal to 34.
  CMD43 pickup        145af4a70 -> begin(0x2b), u32, u8 0, u8, u16 ...
        matches protocol.DecodePickup: u32 object, p[4]==0, u16 ActorX at 6.
  CMD1301 village     143ca6672 -> begin(0x515), send  (empty body)
        matches the implemented empty-body request.

Writer helpers, established by those three:
  146d746e0 begin command   146d75cc0 u8
  146d76180 u16             146d75ce0 u32
  146d75af0 send

A conditional field write appears as a writer call reached only on one branch,
so the linear trace below is a candidate layout, not proof; it still has to
agree with a captured packet before anything is implemented against it.

Usage:  python command_layout36.py 623 283 38 ...
"""
import bisect
import contextlib
import io
import json
import pathlib
import runpy
import sys

p = pathlib.Path(__file__).parent
with contextlib.redirect_stdout(io.StringIO()):
    n = runpy.run_path(str(p / 'decode_literals.py'))
pe, base, md, table, begins = [n[k] for k in ['pe', 'base', 'md', 'table', 'begins']]

WRITERS = {
    0x146d746e0: 'BEGIN',
    0x146d75af0: 'SEND',
    0x146d75cc0: 'u8',
    0x146d76180: 'u16',
    0x146d75ce0: 'u32',
    0x146d74000: '.builder',
}


def function_bounds(addr):
    ix = bisect.bisect_right(begins, addr - base) - 1
    beg, end, _ = table[ix]
    return base + beg, base + end


def trace(site, limit=0x1200):
    """Walk forward from a begin-command call to the send, naming each writer."""
    beg, end = function_bounds(site)
    stop = min(end, site + limit)
    code = list(md.disasm(pe.get_data(site - base, stop - site), site))
    steps = []
    pending = None
    for x in code:
        if x.mnemonic in ('mov', 'movzx') and x.op_str.split(',')[0].strip() in ('edx', 'dx', 'dl', 'rdx'):
            pending = x.op_str.split(',', 1)[1].strip()
        elif x.mnemonic == 'xor' and x.op_str.strip() in ('edx, edx', 'rdx, rdx'):
            pending = '0'
        if x.mnemonic not in ('call', 'jmp') or not x.op_str.startswith('0x'):
            continue
        target = int(x.op_str, 16)
        name = WRITERS.get(target)
        if name is None:
            continue
        if name == '.builder':
            continue
        if name == 'BEGIN':
            steps.append(('BEGIN', pending, x.address))
            pending = None
            continue
        if name == 'SEND':
            steps.append(('SEND', None, x.address))
            break
        steps.append((name, pending, x.address))
        pending = None
    return beg, steps


SIZES = {'u8': 1, 'u16': 2, 'u32': 4}


def scan_all():
    """Emit a layout fingerprint for every command, so a feature can be found
    by the shape of its request instead of by guessing an opcode number."""
    senders = json.loads((p / 'command_senders36.json').read_text(encoding='utf-8'))['commands']
    rows = []
    for cid, sites in senders.items():
        best = None
        for s in sites:
            try:
                _, steps = trace(int(s['at'], 16))
            except Exception:
                continue
            if not steps or steps[0][0] != 'BEGIN':
                continue
            fields = [k for k, _, _ in steps[1:] if k in SIZES]
            size = sum(SIZES[k] for k in fields)
            sent = any(k == 'SEND' for k, _, _ in steps)
            cand = (sent, len(fields), size, ','.join(fields), s['function'])
            # Prefer a trace that actually reached the send.
            if best is None or (cand[0], -cand[1]) > (best[0], -best[1]):
                best = cand
        if best is None:
            continue
        rows.append({'command': int(cid), 'closed': best[0], 'fields': best[1],
                     'body': best[2], 'signature': best[3], 'function': best[4]})
    rows.sort(key=lambda r: r['command'])
    out = p / 'command_layouts36.json'
    out.write_text(json.dumps(rows, indent=2), encoding='utf-8')
    print(f'{"cmd":>6} {"body":>5} {"n":>3} sig')
    for r in rows:
        flag = '' if r['closed'] else '  (open)'
        print(f'{r["command"]:>6} {r["body"]:>5} {r["fields"]:>3} {r["signature"]}{flag}')
    print(f'\nwrote {out.name}  rows={len(rows)}')


def main():
    if sys.argv[1:2] == ['--scan']:
        scan_all()
        return
    senders = json.loads((p / 'command_senders36.json').read_text(encoding='utf-8'))['commands']
    for arg in sys.argv[1:]:
        cid = str(int(arg))
        sites = senders.get(cid)
        print(f'\n===== command {cid}')
        if not sites:
            print('  no resolved sender for this command')
            continue
        for s in sites:
            site = int(s['at'], 16)
            func, steps = trace(site)
            if not steps or steps[0][0] != 'BEGIN':
                continue
            offset = 0
            print(f'  sender {s["function"]}  begin at {site:x}')
            for kind, operand, addr in steps:
                if kind == 'BEGIN':
                    print(f'      begin   {operand}')
                    continue
                if kind == 'SEND':
                    print(f'      send            total body = {offset} bytes')
                    break
                print(f'      +{offset:<4} {kind:<4} {operand}')
                offset += SIZES[kind]
            else:
                print(f'      (no send reached within the traced window; '
                       f'{offset} bytes so far)')


if __name__ == '__main__':
    main()

"""Enumerate every command this exact client build can send.

The send sequence, confirmed against the already-implemented CMD1301 village
return at 143ca6672..143ca668e, is:

    call 146d74000      # obtain the packet builder
    mov  edx, <command> # command identity
    call 146d746e0      # begin command
    ...field writes...
    call/jmp 146d75af0  # send

Walking every call to the begin-command helper and recovering the constant in
edx yields this build's own command table. That is the whole point: another
build's opcode numbering does not transfer, and the project forbids porting
it. Read-only static analysis of the client image.

Usage:  python command_senders36.py [--json out.json]
"""
import bisect
import contextlib
import io
import json
import pathlib
import re
import runpy
import struct
import sys

p = pathlib.Path(__file__).parent
with contextlib.redirect_stdout(io.StringIO()):
    n = runpy.run_path(str(p / 'decode_literals.py'))
pe, base, md, table, begins = [n[k] for k in ['pe', 'base', 'md', 'table', 'begins']]

BEGIN_COMMAND = 0x146d746e0
SEND = 0x146d75af0


def call_sites(target):
    """Every direct call to target in executable sections."""
    for s in pe.sections:
        if not s.Characteristics & 0x20000000:
            continue
        data = s.get_data()
        va = base + s.VirtualAddress
        for m in re.finditer(b'\xe8', data):
            off = m.start()
            if off + 5 > len(data):
                continue
            if va + off + 5 + struct.unpack_from('<i', data, off + 1)[0] != target:
                continue
            yield data, va, off, va + off


def recover_edx(data, va, off, window=64):
    """Recover the command constant placed in edx before the call.

    Only an immediate is accepted. A computed edx (lea/arithmetic) cannot be
    resolved statically and is reported as unknown rather than guessed.
    """
    start = max(0, off - window)
    code = list(md.disasm(data[start:off + 5], va + start))
    value = None
    for x in code:
        dst = x.op_str.split(',')[0].strip()
        if dst not in ('edx', 'dx', 'dl', 'rdx'):
            continue
        if x.mnemonic == 'mov':
            src = x.op_str.split(',', 1)[1].strip()
            if re.fullmatch(r'0x[0-9a-f]+', src):
                value = int(src, 16)
            elif re.fullmatch(r'\d+', src):
                value = int(src)
            else:
                value = None
        elif x.mnemonic == 'xor' and x.op_str.strip() in ('edx, edx', 'rdx, rdx'):
            value = 0
        else:
            value = None
    return value


def enclosing(addr):
    beg, _, _ = table[bisect.bisect_right(begins, addr - base) - 1]
    return base + beg


def main():
    rows = []
    for data, va, off, addr in call_sites(BEGIN_COMMAND):
        rows.append({
            'command': recover_edx(data, va, off),
            'at': addr,
            'function': enclosing(addr),
        })
    known = [r for r in rows if r['command'] is not None]
    unknown = [r for r in rows if r['command'] is None]
    by_cmd = {}
    for r in known:
        by_cmd.setdefault(r['command'], []).append(r)

    print(f'begin-command call sites: {len(rows)}  '
          f'resolved: {len(known)}  computed/unresolved: {len(unknown)}')
    print(f'distinct commands: {len(by_cmd)}\n')
    print(f'{"cmd":>6}  {"sites":>5}  sender functions')
    for cmd in sorted(by_cmd):
        sites = by_cmd[cmd]
        funcs = sorted({r['function'] for r in sites})
        shown = ' '.join(f'{f:x}' for f in funcs[:6])
        if len(funcs) > 6:
            shown += f' (+{len(funcs) - 6})'
        print(f'{cmd:>6}  {len(sites):>5}  {shown}')

    out = p / 'command_senders36.json'
    out.write_text(json.dumps(
        {'begin_command': hex(BEGIN_COMMAND), 'send': hex(SEND),
         'commands': {str(c): [{'at': hex(r['at']), 'function': hex(r['function'])}
                               for r in v] for c, v in sorted(by_cmd.items())},
         'unresolved': [{'at': hex(r['at']), 'function': hex(r['function'])} for r in unknown]},
        indent=2), encoding='utf-8')
    print(f'\nwrote {out.name}')


if __name__ == '__main__':
    main()

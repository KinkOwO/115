"""Find every instruction in this build that touches the monster HP rate field.

145c08420 (hp_revision35.asm) ends with:

    lea  rdx, [rbp + 0x38]
    lea  rcx, [rbx + 0x6808]     # actor + 0x6808
    call 146e920a0               # decode secure field -> local
    ...guard check against [rbx + 0x680c]...
    movss xmm0, [rbp + 0x38]
    movss [rbp + 0x40], xmm0
    lea  rdx, [r14 + 0x30]       # descriptor + 0x30
    lea  rcx, [rbp + 0x40]
    call 146e922e0               # encode local -> secure field

so descriptor+0x30 (the rate the effective-HP getter divides by) is a copy of
actor+0x6808, and 145c08420 only reads it. Whoever writes actor+0x6808 sets
the monster HP scale. Secure fields are always touched through the pair
146e920a0 (decode) / 146e922e0 (encode), so a write is an encode call whose
destination operand is [reg + 0x6808].

Read-only static analysis of the client image. Nothing is patched.

Usage:  python hp_rate_source36.py [--json out.json]
"""
import contextlib
import io
import json
import pathlib
import runpy
import sys

p = pathlib.Path(__file__).parent
with contextlib.redirect_stdout(io.StringIO()):
    n = runpy.run_path(str(p / 'decode_literals.py'))
pe, base, md, table = [n[k] for k in ['pe', 'base', 'md', 'table']]

DECODE = 0x146e920a0
ENCODE = 0x146e922e0
FIELD = 0x6808
GUARD = 0x680c


def code_map():
    """VA -> bytes for every executable section."""
    out = []
    for s in pe.sections:
        if s.Characteristics & 0x20000000:
            out.append((base + s.VirtualAddress, s.get_data()))
    return out


SECTIONS = code_map()


def fetch(va, size):
    for start, data in SECTIONS:
        if start <= va < start + len(data):
            off = va - start
            return data[off:off + size]
    return b''


def functions():
    """(begin_va, end_va) for every .pdata function, deduplicated."""
    seen = set()
    for begin, end, _ in table:
        if begin in seen or end <= begin:
            continue
        seen.add(begin)
        yield base + begin, base + end


def call_target(ins):
    """Direct call/jmp destination, or None."""
    if ins.mnemonic not in ('call', 'jmp'):
        return None
    ops = ins.operands
    if len(ops) != 1 or ops[0].type != capstone.CS_OP_IMM:
        return None
    return ops[0].imm


def touches_field(ins):
    """The field displacement this instruction addresses, or None."""
    for op in ins.operands:
        if op.type == capstone.CS_OP_MEM and op.mem.disp in (FIELD, GUARD):
            return op.mem.disp
    return None


import capstone  # noqa: E402  (decode_literals already imported it)


def scan():
    hits = []
    for begin, end in functions():
        data = fetch(begin, end - begin)
        if not data:
            continue
        code = list(md.disasm(data, begin))
        for i, ins in enumerate(code):
            disp = touches_field(ins)
            if disp is None:
                continue
            follow = None
            for nxt in code[i + 1:i + 7]:
                t = call_target(nxt)
                if t in (DECODE, ENCODE):
                    follow = 'decode' if t == DECODE else 'encode'
                    break
            hits.append({'function': hex(begin), 'address': hex(ins.address),
                         'disp': hex(disp), 'text': f'{ins.mnemonic} {ins.op_str}',
                         'via': follow})
    return hits


def main():
    hits = scan()
    by_function = {}
    for h in hits:
        by_function.setdefault(h['function'], []).append(h)

    writers = {f: rows for f, rows in by_function.items()
               if any(r['via'] == 'encode' for r in rows)}
    readers = {f: rows for f, rows in by_function.items() if f not in writers}

    print(f'{len(hits)} instructions in {len(by_function)} functions address '
          f'+0x6808/+0x680c')
    print(f'\n== {len(writers)} function(s) ENCODE into the field (writers)')
    for f, rows in sorted(writers.items()):
        print(f'  FUNCTION {f}')
        for r in rows:
            print(f'    {r["address"]}  {r["text"]:<44} {r["disp"]}  {r["via"] or ""}')
    print(f'\n== {len(readers)} function(s) only read or guard it')
    for f, rows in sorted(readers.items()):
        print(f'  FUNCTION {f}  ({len(rows)} refs)  '
              + ', '.join(sorted({r['via'] or 'raw' for r in rows})))

    if '--json' in sys.argv:
        out = pathlib.Path(sys.argv[sys.argv.index('--json') + 1])
        out.write_text(json.dumps(hits, indent=1), encoding='utf-8')
        print(f'\nwrote {out}')


if __name__ == '__main__':
    main()

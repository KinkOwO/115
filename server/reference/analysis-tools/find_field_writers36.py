"""Find every function that writes a guarded field at a given offset.

Guarded fields are only ever written through 146e922e0(rcx=source local,
rdx=destination field) - the pairing is visible in 145c08420, which decodes
with 146e920a0 and encodes with 146e922e0. So a write to descriptor+0x30 is
a call to the encoder preceded by `lea rdx, [reg + 0x30]`.

    python find_field_writers36.py 0x30 [0x48 ...]

Read-only static analysis of the client image.
"""
import contextlib
import io
import pathlib
import runpy
import sys

p = pathlib.Path(__file__).parent
with contextlib.redirect_stdout(io.StringIO()):
    n = runpy.run_path(str(p / 'decode_literals.py'))
import capstone  # noqa: E402  (decode_literals puts python_lib on sys.path)
pe, base, md, table = n['pe'], n['base'], n['md'], n['table']

ENCODE = {0x146e922e0, 0x146e922f0}
WANT = {int(a, 0) for a in sys.argv[1:]} or {0x30}
SECTIONS = [(base + s.VirtualAddress, s.get_data())
            for s in pe.sections if s.Characteristics & 0x20000000]


def fetch(va, size):
    for start, data in SECTIONS:
        if start <= va < start + len(data):
            return data[va - start:va - start + size]
    return b''


RDX = capstone.x86.X86_REG_RDX
hits = []
seen_fn = set()
for begin, end, _ in table:
    if begin in seen_fn or end <= begin:
        continue
    seen_fn.add(begin)
    va = base + begin
    data = fetch(va, end - begin)
    if not data:
        continue
    code = list(md.disasm(data, va))
    for i, ins in enumerate(code):
        if ins.mnemonic != 'call' or len(ins.operands) != 1:
            continue
        op = ins.operands[0]
        if op.type != capstone.CS_OP_IMM or op.imm not in ENCODE:
            continue
        # Walk back for the destination operand placed in rdx.
        for prev in reversed(code[max(0, i - 8):i]):
            if prev.mnemonic != 'lea' or len(prev.operands) != 2:
                continue
            dst, src = prev.operands
            if dst.type != capstone.CS_OP_REG or dst.reg != RDX:
                continue
            if src.type == capstone.CS_OP_MEM and src.mem.disp in WANT:
                hits.append((hex(va), hex(ins.address), prev.op_str))
            break

print(f'{len(hits)} encoder writes to field offset(s) '
      + ', '.join(hex(w) for w in sorted(WANT)))
for fn, at, text in hits:
    print(f'  FUNCTION {fn}  at {at}  lea rdx, {text.split(", ", 1)[1]}')

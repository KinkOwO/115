"""Bounded read-only disassembly of explicit current-build functions."""
import bisect, contextlib, io, pathlib, runpy, sys

root = pathlib.Path(__file__).parent
with contextlib.redirect_stdout(io.StringIO()):
    env = runpy.run_path(str(root / 'decode_literals.py'))
pe, md, base, table, begins = (env[k] for k in ('pe', 'md', 'base', 'table', 'begins'))
lines = []
for spec in sys.argv[2:]:
    parts = spec.split(':')
    start = int(parts[0], 16)
    if len(parts) == 2:
        size = int(parts[1], 16)
    else:
        begin, end, _ = table[bisect.bisect_right(begins, start-base)-1]
        size = end-(start-base)
    if not 0 < size <= 0x20000:
        raise ValueError('invalid region size')
    ins = list(md.disasm(pe.get_data(start-base, size), start))
    lines.append(f'FUNCTION {start:x} SIZE {size:x}')
    for i, x in enumerate(ins):
        line = f'{x.address:x} {x.mnemonic} {x.op_str}'
        if x.mnemonic == 'call' and x.op_str in ('0x146e8c7d0','0x146e8c7b0') and i:
            prev = ins[i-1]
            if prev.mnemonic == 'lea' and prev.op_str.startswith('rcx, [rip'):
                target = prev.address + prev.size + prev.operands[1].mem.disp
                try:
                    line += ' ; ' + env['decode'](target, 2 if x.op_str.endswith('7d0') else 1).replace('\n', '\\n')
                except (ValueError, UnicodeError):
                    pass
        lines.append(line)
dest = root / sys.argv[1]
if dest.parent != root or dest.suffix != '.asm':
    raise ValueError('output must be an asm filename in the probe directory')
dest.write_text('\n'.join(lines)+'\n', encoding='utf-8')
print(f'{dest.name}: {len(lines)} lines')

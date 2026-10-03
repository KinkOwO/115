import re, zlib, sys

base = r'D:\115us\analysis-tools\output\official_20261002-160349_decoded'
data = open(base + r'\session_s4_s2c.bin', 'rb').read()

pat = re.compile(r's2c\s+(\d+)\s+@0x([0-9A-F]+)\s+kind=(\d)\s+id=(\d+)\s+size=(\d+)\s+body=(\d+)')
rows = []
for ln in open(base + r'\session_s4_s2c.txt', encoding='utf-8-sig'):
    m = pat.search(ln)
    if m:
        rows.append((int(m.group(1)), int(m.group(2), 16), int(m.group(4)), int(m.group(5)), int(m.group(6))))

want_ids = {12, 13, 14, 23, 24}
for frame, off, oid, size, body in rows:
    if oid in want_ids and body > 8:
        raw = data[off:off+size]
        b = raw[16:16+body] if len(raw) >= 16+body else raw
        out = b
        note = ''
        if b[:2] == b'\x78\x9c':
            try:
                out = zlib.decompress(b)
                note = f'zlib {len(b)} -> {len(out)}'
            except Exception as e:
                note = f'zlib fail {e}'
        print(f'--- frame {frame} op={oid} body={body} {note}')
        print(out[:260].hex(' '))
        # printable strings
        txt = ''.join(chr(c) if 32 <= c < 127 else '.' for c in out[:260])
        print('   txt:', txt[:200])

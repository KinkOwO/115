lines = open(r'd:\115us\115-server\server\work\dfo-lan\cmd\wireprobe\_dump781_out.txt', encoding='utf-8-sig').read().splitlines()

def flush(sess, frame, op, hexchunks):
    if op != 781 or not hexchunks:
        return
    b = bytes.fromhex(''.join(hexchunks))
    if len(b) < 272:
        return
    recs = list(b[12::8])
    print(f"{sess} f{frame} 781 state@4={b[4:8].hex()} 0xff={b[0xff]:02x} tail={b[-16:].hex()}")
    print(f"   recs={recs}")

sess = frame = op = None
chunks = []
for ln in lines:
    s = ln.strip()
    if s.startswith('['):
        flush(sess, frame, op, chunks)
        parts = s.split()
        opp = next((p for p in parts if p.startswith('op=')), None)
        if opp is None:
            sess = frame = op = None
            chunks = []
            continue
        sess, frame, op = parts[0].strip('[]'), int(parts[2]), int(opp.split('=')[1])
        chunks = []
    elif s and all(c in '0123456789abcdef' for c in s):
        chunks.append(s)
flush(sess, frame, op, chunks)

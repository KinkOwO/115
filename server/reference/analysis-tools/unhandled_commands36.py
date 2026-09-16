"""Survey every client command seen in past runs and which ones the server
never answered. Read-only: it only reads events.jsonl capture logs.

A command the client sent but the server never replied to is exactly where an
unimplemented feature shows up, so this is the evidence base for mail,
avatars and item use rather than porting another build's opcodes.
"""
import json
import pathlib
import sys
from collections import defaultdict

runtime = pathlib.Path(__file__).parent.parent / 'dfo-lan' / 'runtime'
sent = defaultdict(int)          # client -> server, by command id
answered = defaultdict(int)      # server replies, by command id
bodies = {}                      # first plaintext body seen per command
per_run = defaultdict(set)
sizes = defaultdict(lambda: defaultdict(int))   # command -> frame length -> count

for log in sorted(runtime.glob('*/events.jsonl')):
    if not log.stat().st_size:
        continue
    run = log.parent.name
    for line in log.read_text(encoding='utf-8', errors='replace').splitlines():
        try:
            r = json.loads(line)
        except Exception:
            continue
        kind = r.get('kind')
        if kind == 'client_frame':
            cid = r.get('id')
            if cid is None:
                continue
            sent[cid] += 1
            per_run[cid].add(run)
            # Frame length survives even when the body was discarded, and the
            # payload size is a usable fingerprint: a 13-byte header plus a
            # 16-byte padded body is a 29-byte frame.
            if r.get('bytes'):
                sizes[cid][r['bytes']] += 1
            if cid not in bodies and r.get('plain_hex'):
                bodies[cid] = (r['plain_hex'], r.get('bytes'), r.get('checksum_ok'))
        elif r.get('id') is not None and kind not in ('client_frame', 'accept', 'close'):
            answered[r['id']] += 1

# Commands this build explicitly handles, from cmd/wireprobe/main.go dispatch.
handled = {1, 2, 3, 4, 5, 6, 7, 8, 15, 16, 19, 28, 29, 31, 32, 33, 34, 35, 36,
           37, 39, 42, 43, 45, 46, 69, 70, 71, 72, 117, 132, 143, 191, 433,
           637, 684, 848, 1301}

print('== client commands seen across all captured runs')
print(f'{"cmd":>6} {"sent":>6} {"replies":>7}  {"runs":>4}  status')
for cid in sorted(sent):
    mark = 'handled' if cid in handled else 'NO HANDLER'
    if cid not in handled and answered.get(cid):
        mark = 'fixture/響 only'
    print(f'{cid:>6} {sent[cid]:>6} {answered.get(cid, 0):>7}  {len(per_run[cid]):>4}  {mark}')

print('\n== unhandled commands by payload size')
print('   frame = 13-byte header + padded body, so body = frame - 13.')
print('   A 16-byte body (frame 29) matches the reference use-stackable shape')
print('   "i16 slot | u8 list | i32 instance | i32 item | u32 reserved".')
print(f'{"cmd":>6} {"sent":>6}  body sizes (count)')
for cid in sorted(sent):
    if cid in handled:
        continue
    parts = ' '.join(f'{n - 13}B:{c}' for n, c in sorted(sizes[cid].items()))
    print(f'{cid:>6} {sent[cid]:>6}  {parts}')

print('\n== unhandled commands with a decrypted body (candidate features)')
for cid in sorted(sent):
    if cid in handled:
        continue
    b = bodies.get(cid)
    if not b:
        print(f'  cmd {cid:>6} sent={sent[cid]:<4} (body not retained by the observer whitelist)')
        continue
    hexbody, nbytes, ok = b
    print(f'  cmd {cid:>6} sent={sent[cid]:<4} frame_bytes={nbytes} checksum_ok={ok}')
    print(f'         plain={hexbody}')

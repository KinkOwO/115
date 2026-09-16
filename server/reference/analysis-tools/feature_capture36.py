"""Turn one ordinary play session into protocol evidence for the features this
build does not implement yet: mail, avatars, the shop and item use.

Read-only. Run it after a session in which the listed UI actions were done;
36's observer samples up to BodySampleLimit bodies per command, so each action
leaves a decrypted payload behind.

    python dfo_probe_tools/feature_capture36.py [run-directory]

With no argument it picks the newest run that has a capture log.
"""
import json
import pathlib
import sys
from collections import defaultdict

runtime = pathlib.Path(__file__).parent.parent / 'dfo-lan' / 'runtime'

HANDLED = {0, 1, 2, 3, 4, 5, 6, 7, 8, 15, 16, 19, 28, 29, 31, 32, 33, 34, 35,
           36, 37, 39, 42, 43, 45, 46, 69, 70, 71, 72, 117, 132, 143, 191, 433,
           623, 627, 637, 684, 848, 1301, 1554}


def newest_run():
    runs = [p.parent for p in runtime.glob('*/events.jsonl') if p.stat().st_size]
    if not runs:
        raise SystemExit('no capture log found under runtime/')
    return max(runs, key=lambda p: (p / 'events.jsonl').stat().st_mtime)


def main():
    run = pathlib.Path(sys.argv[1]) if len(sys.argv) > 1 else newest_run()
    log = run / 'events.jsonl'
    print(f'run: {run.name}')

    samples = defaultdict(list)   # command id -> [(time, plain, frame bytes)]
    counts = defaultdict(int)
    for line in log.read_text(encoding='utf-8', errors='replace').splitlines():
        try:
            r = json.loads(line)
        except Exception:
            continue
        if r.get('kind') != 'client_frame':
            continue
        cid = r.get('id')
        if cid is None:
            continue
        counts[cid] += 1
        if r.get('plain_hex') and cid not in HANDLED:
            samples[cid].append((r.get('time', ''), r['plain_hex'], r.get('bytes')))

    if not samples:
        print('\nNo unimplemented-command bodies in this log.')
        print('Either the run predates 36 (its observer discarded them), or the')
        print('UI actions below were not performed during the session.')
        return

    print(f'\n== {len(samples)} unimplemented commands captured with payloads\n')
    for cid in sorted(samples, key=lambda c: -counts[c]):
        rows = samples[cid]
        print(f'command {cid}  sent={counts[cid]}  bodies={len(rows)}')
        widths = {len(p) // 2 for _, p, _ in rows}
        print(f'  plaintext length(s): {sorted(widths)}')
        for t, p, n in rows:
            print(f'  {t[11:23]:>12}  frame={n:<5} {p}')
        print()

    print('== how to read this')
    print('  A command that appears right after one UI action, with a payload')
    print('  whose length is stable across repeats, is that action\'s request.')
    print('  Correlate the timestamps with the order the actions were done:')
    print('    open the mail window          -> mail list request')
    print('    take one mail attachment      -> mail claim request (carries a mail id)')
    print('    open the shop                 -> shop listing request')
    print('    buy one cheap item           -> purchase request (carries an item id)')
    print('    use one potion from the bag   -> item use request (carries a bag slot)')
    print('    move an avatar piece onto the')
    print('    avatar page                  -> avatar slot numbers, via CMD19 or its own command')
    print()
    print('  Field widths then come from the payload itself, and the server side')
    print('  is implemented against this client\'s own format instead of another')
    print('  build\'s opcode table.')


if __name__ == '__main__':
    main()

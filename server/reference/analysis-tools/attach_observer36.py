"""Attach the read-only stat observer to the 36 run the launcher just made.

channel_probe only auto-starts the observer for the _next30.._next34 tags, and
a 36 tag is rewritten to _next35 before that test, so an interactive 36 session
runs without one. This waits for the next run directory to appear and starts
watch_monster_stats36.py against it. Read-only: it opens the owned client with
PROCESS_VM_READ only and writes nothing into the game.

    python dfo_probe_tools/attach_observer36.py
"""
import pathlib
import subprocess
import sys
import time

here = pathlib.Path(__file__).resolve().parent
runtime = here.parent / 'dfo-lan' / 'runtime'
known = {p.name for p in runtime.glob('*_next36')}
print(f'waiting for a new _next36 run ({len(known)} already present)')

deadline = time.time() + 900
run = None
while time.time() < deadline:
    fresh = [p for p in runtime.glob('*_next36') if p.name not in known]
    if fresh:
        run = max(fresh, key=lambda p: p.stat().st_mtime)
        break
    time.sleep(0.5)
if run is None:
    raise SystemExit('no new run directory appeared within 15 minutes')

print(f'run: {run.name}')
observer = here / 'watch_monster_stats36.py'
out = run / 'observer36.out'
err = run / 'observer36.err'
with out.open('wb') as o, err.open('wb') as e:
    child = subprocess.Popen([sys.executable, str(observer), str(run)],
                             stdout=o, stderr=e,
                             creationflags=subprocess.CREATE_NO_WINDOW)
print(f'observer pid {child.pid} -> {run / "monster-stats.jsonl"}')
print('it waits up to 60s for the client PID, then samples every 0.5s.')

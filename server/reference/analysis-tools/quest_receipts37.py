"""What did completed quests actually pay out? Read the stored receipts.

Ground truth for "rewards only give experience": every settled quest commits
a receipt in character_quest_rewards. Prints receipts and the character's
current gold/bag summary. Credentials stay inside this process.

    python quest_receipts37.py [character-id]
"""
import json
import pathlib
import subprocess
import sys

root = pathlib.Path(__file__).parent.parent / 'dfo-lan'
cfg = json.loads((root / 'runtime/storage/local.json').read_text(encoding='utf-8'))
psql = pathlib.Path(cfg['postgres_bin']) / 'psql.exe'
who = sys.argv[1] if len(sys.argv) > 1 else '3'


def q(sql):
    r = subprocess.run([str(psql), cfg['postgres_dsn'], '-X', '-A', '-t', '-c', sql],
                       capture_output=True, text=True, timeout=30)
    if r.returncode:
        raise SystemExit('query failed: ' + r.stderr.strip()[:200])
    return r.stdout.strip()


print('== quest reward receipts for character', who)
rows = q(f"SELECT receipt FROM character_quest_rewards WHERE character_id={int(who)} ORDER BY quest_id")
for line in rows.splitlines():
    try:
        r = json.loads(line)
    except Exception:
        continue
    items = r.get('items') or []
    detail = ', '.join(f'{i["Template"]}x{i["Amount"]}@{i.get("Slots")}' for i in items) or 'none'
    print(f'  quest {r.get("quest"):>6}  exp {r.get("experience"):>6}  items: {detail}')

print('\n== bag summary')
state = q(f"SELECT state::text FROM characters WHERE id={int(who)}")
s = json.loads(state)
inv = s.get('inventory', {})
print('  gold:', inv.get('gold'))
print('  stacks:', [(i.get('slot'), i.get('Template'), i.get('Amount')) for i in inv.get('items') or []])
print('  equipment:', [(i.get('slot'), i.get('template'), i.get('durability')) for i in inv.get('equipment') or []])
print('  worn:', [(i.get('slot'), i.get('template')) for i in inv.get('worn') or []])
print('  level:', s.get('level'), 'advancement:', s.get('advancement'))

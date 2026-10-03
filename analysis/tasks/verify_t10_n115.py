import json

d = r"d:\115us\115-server\server\work\dfo-lan\runtime\roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261003_134327_943975_next37"
lines = open(d + r"\events.jsonl", encoding="utf-8", errors="replace").read().splitlines()
events = [json.loads(l) for l in lines]

# 1) all official s2c id=115 frames with full bodies
print("=== official s2c id=115 frames ===")
for l in open(r"d:\115us\analysis-tools\output\official_20261002-160349_decoded\session_s4_s2c_plain.txt", encoding="utf-8", errors="replace"):
    if " id=115 " in l:
        fr = int(l.split()[0].split("=")[1])
        plain = l.split("plain=", 1)[1].strip() if "plain=" in l else ""
        print("f%-4d %s" % (fr, plain))

# our N115 body
ph = events[798].get("plain_hex") or ""
print("ours(t10 ev798):", ph)

# 2) our 585/117/46 vs official stage-0 c2s frames 336/337/338
off_c2s = {}
for l in open(r"d:\115us\analysis-tools\output\official_20261002-160349_decoded\session_s4_c2s_plain.txt", encoding="utf-8", errors="replace"):
    if not l.startswith("frame="):
        continue
    parts = dict(p.split("=", 1) for p in l.split() if "=" in p)
    fr = int(parts["frame"])
    plain = l.split("plain=", 1)[1].strip() if "plain=" in l else ""
    off_c2s[fr] = bytes.fromhex(plain)

def cmpb(label, a, b):
    if a == b:
        print("%s: BYTE-EQUAL" % label)
        return
    n = min(len(a), len(b))
    diffs = [i for i in range(n) if a[i] != b[i]]
    print("%s: DIFF len %d vs %d, diffs %s" % (label, len(a), len(b), diffs[:15]))
    for i in diffs[:6]:
        print("   off %d: ours %s | official %s" % (i, a[max(0,i-3):i+5].hex(), b[max(0,i-3):i+5].hex()))

ours585 = bytes.fromhex(events[799]["plain_hex"])
ours117 = bytes.fromhex(events[800]["plain_hex"])
ours46 = bytes.fromhex(events[802]["plain_hex"])
cmpb("c 585 (vs off c2s 336, env-stripped)", ours585, off_c2s[336][13:])
cmpb("c 117 (vs off c2s 337)", ours117, off_c2s[337][13:])
cmpb("c 46 (vs off c2s 338)", ours46, off_c2s[338][13:])
print()
print("ours 2060:", events[801]["plain_hex"])
print("official c2s 339 (1654):", off_c2s[339][13:].hex())

# 3) our earlier id=115 events in this run (all boss_check frames)
print()
print("=== all our id=115 outbound events (t10) ===")
for i, e in enumerate(events):
    if e.get("id") == 115 and e.get("plain_hex") is not None:
        print(i, e.get("time", "")[11:23], e.get("kind"), e["plain_hex"][:60])

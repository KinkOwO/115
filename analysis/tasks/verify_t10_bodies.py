import json, zlib

d = r"d:\115us\115-server\server\work\dfo-lan\runtime\roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261003_134327_943975_next37"
lines = open(d + r"\events.jsonl", encoding="utf-8", errors="replace").read().splitlines()
events = [json.loads(l) for l in lines]

def off(ev):
    ph = ev.get("plain_hex") or ""
    return bytes.fromhex(ph)

# our chain events by index
ours = {i: events[i] for i in range(770, 805)}

# official bodies by frame number (s2c)
off_s2c = {}
for l in open(r"d:\115us\analysis-tools\output\official_20261002-160349_decoded\session_s4_s2c_plain.txt", encoding="utf-8", errors="replace"):
    if not l.startswith("frame="):
        continue
    parts = dict(p.split("=", 1) for p in l.split() if "=" in p)
    fr = int(parts["frame"])
    plain = l.split("plain=", 1)[1].strip() if "plain=" in l else ""
    off_s2c[fr] = bytes.fromhex(plain)

# official c2s bodies
off_c2s = {}
for l in open(r"d:\115us\analysis-tools\output\official_20261002-160349_decoded\session_s4_c2s_plain.txt", encoding="utf-8", errors="replace"):
    if not l.startswith("frame="):
        continue
    parts = dict(p.split("=", 1) for p in l.split() if "=" in p)
    fr = int(parts["frame"])
    plain = l.split("plain=", 1)[1].strip() if "plain=" in l else ""
    off_c2s[fr] = bytes.fromhex(plain)

def cmp(label, a, b, limit=48):
    if a == b:
        print("%s: BYTE-EQUAL (%d B)" % (label, len(a)))
        return
    n = min(len(a), len(b))
    diffs = [i for i in range(n) if a[i] != b[i]]
    print("%s: DIFF len %d vs %d; first diff offsets %s" % (label, len(a), len(b), diffs[:12]))
    for i in diffs[:4]:
        lo = max(0, i - 4)
        print("   off %d: ours %s | official %s" % (i, a[lo:i+6].hex(), b[lo:i+6].hex()))
    if len(a) != len(b):
        print("   tail ours  : %s" % a[n:n+24].hex())
        print("   tail offici: %s" % b[n:n+24].hex())

# 1) our outbound chain vs official (aligned pairs)
pairs = [
    ("38 monster_death", 771, 468),
    ("2204", 772, 469), ("2201", 773, 470), ("279", 774, 471),
    ("31", 775, 472), ("2256", 776, 473),
    ("2168 a", 777, 474), ("14 a", 781, 478),
    ("2252", 782, 479), ("2168 b", 783, 480), ("14 b", 786, 483),
    ("N2", 791, 488), ("2253", 792, 489), ("2254", 790 + 4 - 1, 490),  # 793
    ("N9", 794, 491), ("2255", 795, 492), ("1658", 796, 493),
    ("2168 c", 797, 494), ("115", 798, 495),
]
pairs[12] = ("2254", 793, 490)
for label, ei, fr in pairs:
    cmp(label + " (t10 ev%d vs off f%d)" % (ei, fr), off(events[ei]), off_s2c[fr])

# 2) zlib frames decompressed compare
print()
for label, ei, fr in [("2252", 782, 479), ("2253", 792, 489)]:
    try:
        a = zlib.decompress(off(events[ei]))
        b = zlib.decompress(off_s2c[fr])
        cmp("%s decompressed" % label, a, b)
    except Exception as ex:
        print("%s decompress failed: %s" % (label, ex))

# 3) client-sent frames: ours vs official (strip 13B envelope from official)
print()
for label, ei, fr in [("c 585", 799, 487), ("c 117", 800, 488), ("c 46", 802, 489), ("c 2060", 801, None)]:
    o = off(events[ei])
    if fr is None:
        print("%s (ours only, %d B): %s" % (label, len(o), o.hex()))
        continue
    f = off_c2s[fr][13:]
    cmp("%s (t10 ev%d vs off c2s f%d)" % (label, ei, fr), o, f)

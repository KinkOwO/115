import json

d = r"d:\115us\115-server\server\work\dfo-lan\runtime\roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261003_134327_943975_next37"
events = [json.loads(l) for l in open(d + r"\events.jsonl", encoding="utf-8", errors="replace")]
n2 = bytes.fromhex(events[791]["plain_hex"])

off_s2c = {}
for l in open(r"d:\115us\analysis-tools\output\official_20261002-160349_decoded\session_s4_s2c_plain.txt", encoding="utf-8", errors="replace"):
    if not l.startswith("frame="):
        continue
    fr = int(l.split()[0].split("=")[1])
    plain = l.split("plain=", 1)[1].strip() if "plain=" in l else ""
    off_s2c[fr] = bytes.fromhex(plain)
on2 = off_s2c[488]

# Official layout: name1@32 (5B), post-name1@37; actor@165, name2@171 (5B), post-name2@176.
# Ours (3B names): name1@32 (3B), post-name1@35; actor@163, name2@169 (3B), post-name2@172.
# So any official offset off maps to ours off - (5-3) = off - 2 after name1,
# and off - (5-3)*2 = off - 4 after name2.
def ours_off(off):
    if off < 37:
        return off
    if off < 165:
        return off - 2
    return off - 4

diffs = []
for i in range(min(len(n2), len(on2))):
    if n2[i] != on2[i]:
        diffs.append(i)

# Classify: which diffs are NOT explained by name/actor substitution?
# Expected diff regions in official offsets:
#   28..31 (name1 len), 32..36 (name1), 165..166 (actor), 167..170 (name2 len), 171..175 (name2),
#   416..419 (stage patch)
name_regions = [(28,31),(32,36),(165,166),(167,170),(171,175),(416,419)]
def in_expected(off_off):
    for a,b in name_regions:
        if a <= off_off <= b:
            return True
    return False

unexpected = []
for o in diffs:
    # map ours offset -> official offset
    if o < 37:
        ooff = o
    elif o < 163:
        ooff = o + 2
    else:
        ooff = o + 4
    if not in_expected(ooff):
        unexpected.append((o, ooff, n2[o], on2[ooff]))

print("total diffs:", len(diffs))
print("unexpected diffs (ours_off, off_off, ours_val, off_val):")
for u in unexpected:
    print("  ", u)

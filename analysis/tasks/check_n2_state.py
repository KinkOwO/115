import json, zlib

d = r"d:\115us\115-server\server\work\dfo-lan\runtime\roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261003_134327_943975_next37"
events = [json.loads(l) for l in open(d + r"\events.jsonl", encoding="utf-8", errors="replace")]

n2 = bytes.fromhex(events[791]["plain_hex"])
print("our N2 len", len(n2))
# our name1 = "001" (3 bytes), official name1 = "yan55" (5 bytes).
# Official layout: name1 len@28(u32), name@32(5B), so post-name1 starts at 37.
# Ours: name@32(3B), post-name1 starts at 35. Offset shift = -2.
shift = -2
for off_official in [180, 416, 417, 418, 419]:
    off_ours = off_official + shift
    print("  official off %d -> ours off %d = %02x" % (off_official, off_ours, n2[off_ours]))

# dump around the state byte
print("ours N2[170:185]:", n2[170:185].hex())
print("ours N2[410:420]:", n2[410:420].hex())

# official N2 (f488) from s2c
import re
off_s2c = {}
for l in open(r"d:\115us\analysis-tools\output\official_20261002-160349_decoded\session_s4_s2c_plain.txt", encoding="utf-8", errors="replace"):
    if not l.startswith("frame="):
        continue
    fr = int(l.split()[0].split("=")[1])
    plain = l.split("plain=", 1)[1].strip() if "plain=" in l else ""
    off_s2c[fr] = bytes.fromhex(plain)

on2 = off_s2c[488]
print()
print("official N2 len", len(on2))
print("official N2[170:185]:", on2[170:185].hex())
print("official N2[410:420]:", on2[410:420].hex())
print("official N2 byte180 =", on2[180])

# full diff of N2 excluding name regions
diffs = []
for i in range(min(len(n2), len(on2))):
    if n2[i] != on2[i]:
        diffs.append(i)
print()
print("N2 diff offsets:", diffs[:30])

import json

d = r"d:\115us\115-server\server\work\dfo-lan\runtime\roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261003_134327_943975_next37"
lines = open(d + r"\events.jsonl", encoding="utf-8", errors="replace").read().splitlines()

# 1) our full outbound chain around settlement: find the window from boss kill to client 2060
events = []
for i, l in enumerate(lines):
    try:
        events.append((i, json.loads(l)))
    except Exception:
        pass

# find client_frame id=2060 index
crash_idx = None
for i, e in events:
    if e.get("id") == 2060:
        crash_idx = i
        break
print("client 2060 at event index", crash_idx)

# walk backwards to find chain start: first outbound after last big gap or after boss death marker
# print all events from index crash_idx-40 to crash_idx+3
print("=== our chain window (t10) ===")
for i, e in events[max(0, crash_idx - 40): crash_idx + 4]:
    t = e.get("time", "")
    kind = e.get("kind", "")
    idv = e.get("id")
    ph = e.get("plain_hex") or ""
    print(i, t[11:23], "id=%s" % idv, "kind=%s" % kind, "len=%d" % (len(ph) // 2))

# 2) official s2c chain: frames 460..500
print()
print("=== official s2c frames 460..505 ===")
for l in open(r"d:\115us\analysis-tools\output\official_20261002-160349_decoded\session_s4_s2c_plain.txt", encoding="utf-8", errors="replace"):
    parts = dict(p.split("=", 1) for p in l.split() if "=" in p and not p.startswith("plain"))
    try:
        fr = int(parts["frame"])
    except Exception:
        continue
    if 460 <= fr <= 505:
        plain = l.split("plain=", 1)[1].strip() if "plain=" in l else ""
        print(l.split("plain=")[0].strip(), "plainlen=%d" % (len(plain) // 2))

# 3) official: any id=2168 or id=14 in s2c settlement window frames 440..520?
print()
print("=== official s2c id=2168 / id=14 frames ===")
for l in open(r"d:\115us\analysis-tools\output\official_20261002-160349_decoded\session_s4_s2c_plain.txt", encoding="utf-8", errors="replace"):
    if "id=2168 " in l or "id=2168\n" in l:
        print("2168:", l.split("plain=")[0].strip())
    if " id=14 " in l:
        fr = l.split()[0].split("=")[1]
        if 400 <= int(fr) <= 600:
            print("14:", l.split("plain=")[0].strip())

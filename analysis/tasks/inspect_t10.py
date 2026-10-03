import json, sys

d = r"d:\115us\115-server\server\work\dfo-lan\runtime\roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261003_134327_943975_next37"
lines = open(d + r"\events.jsonl", encoding="utf-8", errors="replace").read().splitlines()

idx = None
for i, l in enumerate(lines):
    if "ispins_settlement_character_info" in l:
        idx = i
        break
print("first settlement chain line index:", idx)
if idx is None:
    sys.exit(0)

for l in lines[max(0, idx - 3): idx + 40]:
    try:
        e = json.loads(l)
    except Exception:
        print("RAW", l[:150])
        continue
    t = e.get("time", "")
    body = e.get("body") or e.get("plain") or ""
    print(t[11:23], "dir=%s" % e.get("dir"), "id=%s" % e.get("id"), "size=%s" % e.get("size"),
          "kind=%s" % str(e.get("kind")), "body[:72]=%s" % str(body)[:72])

print()
print("=== all events with id in {2, 9, 2254, 1658, 2046, 1654, 2060, 46, 682} ===")
for i, l in enumerate(lines):
    try:
        e = json.loads(l)
    except Exception:
        continue
    if e.get("id") in (2, 9, 2254, 1658, 2046, 1654, 2060, 46, 682):
        t = e.get("time", "")
        body = e.get("body") or e.get("plain") or ""
        print(i, t[11:23], "dir=%s" % e.get("dir"), "id=%s" % e.get("id"),
              "size=%s" % e.get("size"), "note=%s" % e.get("note", ""), "body[:64]=%s" % str(body)[:64])

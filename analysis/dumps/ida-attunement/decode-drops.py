# -*- coding: utf-8 -*-
"""解码本场 4 次有掉落的 monster_death_confirmed（NOTI38）载荷。

行格式（本仓 internal/game/protocol/drop_items.go + inventory.go）：
  u16 entity + u16 count + count × [u32 Object][181B Item][u32 Aux][u16 Sentinel][u16 Owner] + 00 00 ff 00
Item 内：slot[0:2] template[2:6] Data[6:10]（Data = 对玩家的数量）
"""
import json
import struct

EV = r"D:\115us\server\work\dfo-lan\runtime\roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261007_210421_873031_next37\events.jsonl"
GEN = r"D:\115us\server\work\dfo-lan\configs\attunement-rewards.generated.json"

rows = [json.loads(l) for l in open(EV, encoding="utf-8")]
big = [r for r in rows if r.get("kind") == "monster_death_confirmed" and len(r["plain_hex"]) // 2 > 100]
print("有掉落的死亡帧: %d" % len(big))

# 模板 -> (表, 档位) 反查
gen = json.load(open(GEN, encoding="utf-8"))
tier_of = {}
for t in gen["tables"]:
    path = t.get("path", "")

    def walk(node, tier=None):
        if isinstance(node, dict):
            cur = node.get("tier", tier)
            if "item" in node and isinstance(node["item"], int):
                tier_of.setdefault(node["item"], (path.split("/")[-1], node.get("tier", tier)))
            for v in node.values():
                walk(v, cur)
        elif isinstance(node, list):
            for v in node:
                walk(v, tier)

    walk(t)
print("反查表大小 = %d" % len(tier_of))

for r in big:
    b = bytes.fromhex(r["plain_hex"])
    ent, cnt = struct.unpack_from("<HH", b, 0)
    print("\n=== %s entity=%d count=%d ===" % (r["time"], ent, cnt))
    off = 4
    for i in range(cnt):
        obj = struct.unpack_from("<I", b, off)[0]
        item = b[off + 4:off + 4 + 181]
        aux = struct.unpack_from("<I", b, off + 4 + 181)[0]
        slot, tpl, data = struct.unpack_from("<HII", item, 0)
        tail = struct.unpack_from("<HH", b, off + 4 + 181 + 4)
        t, tier = tier_of.get(tpl, ("?", "?"))
        print("  [%d] obj=%d slot=%d template=%-10d data=%-4d aux=%d sent=%d owner=%d  <- %s / %s" % (
            i, obj, slot, tpl, data, aux, tail[0], tail[1], t, tier))
        off += 193

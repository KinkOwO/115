import json
loot = json.load(open(r"D:\115us\gm-tool\configs\loot.next25.json", encoding="utf-8"))
items = loot.get("items") or {}
print("items keys:", len(items))
first = list(items.items())[:2]
for k, v in first:
    print(k, "=>", json.dumps(v, ensure_ascii=False)[:400])
# 找出 kind 字段的值分布
from collections import Counter
kinds = Counter()
stackable_ids = []
for k, v in items.items():
    kinds[v.get("kind", "")] += 1
    if v.get("kind") == "stackable":
        stackable_ids.append(k)
print("kind 分布:", dict(kinds))
print("stackable 示例:", stackable_ids[:10])

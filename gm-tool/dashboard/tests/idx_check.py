import json
idx = json.load(open(r"D:\115us\gm-tool\configs\items.index.json", encoding="utf-8"))
print("top keys:", list(idx.keys())[:8])
items = idx.get("items") or {}
print("items type:", type(items).__name__, "len:", len(items) if hasattr(items, "__len__") else "?")
if isinstance(items, dict):
    keys = list(items.keys())
    print("first keys:", keys[:5])
    hit = items.get("108000001")
    if hit is None:
        hit = items.get(108000001)
    print("108000001:", json.dumps(hit, ensure_ascii=False)[:300])
    like108 = [k for k in keys if str(k).startswith("108")]
    print("108* keys:", like108[:5])
    if like108:
        print("sample:", json.dumps(items[like108[0]], ensure_ascii=False)[:300])
else:
    arr = items
    print("arr len:", len(arr))
    m = [x for x in arr if str(x.get("id")) == "108000001" or str(x.get("template")) == "108000001"]
    print("match:", json.dumps(m, ensure_ascii=False)[:300] if m else "none")

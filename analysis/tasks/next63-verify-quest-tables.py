# -*- coding: utf-8 -*-
"""核对 odyssey-growth-release.json definition.cells 中 quest 四张表（next63 前置取证）"""
import json, sys, io
sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding="utf-8")

d = json.load(open(r"D:/115us/server/work/dfo-lan/configs/odyssey-growth-release.json", encoding="utf-8"))
cells = d["definition"]["cells"]

# token 流: {"type":3,...,"text":"[tag]"} 字符串标记, {"type":0,"value":N} 数字
strs = [(i, c["text"]) for i, c in enumerate(cells) if c.get("type") == 3]
print("top-level string tokens:")
for i, t in strs:
    print(" ", i, t)

# 解析 [quest clear] 下的 [level] 表
def parse_levels(start, end):
    """返回 [(level, [quest ids...]), ...]，范围 [start+1, end)"""
    out, i = [], start + 1
    cur = None
    while i < end:
        c = cells[i]
        if c.get("type") == 3:
            if c["text"] == "[level]":
                cur = {"level": None, "ids": []}
                out.append(cur)
            elif c["text"] == "[/level]":
                cur = None
        elif cur is not None and c.get("type") == 0:
            if cur["level"] is None:
                cur["level"] = c["value"]
            else:
                cur["ids"].append(c["value"])
        i += 1
    return out

idx = {t: i for i, t in strs}
qc = parse_levels(idx["[quest clear]"], idx["[/quest clear]"])
print("\n[quest clear] levels:", [(r["level"], len(r["ids"])) for r in qc])
total = sum(len(r["ids"]) for r in qc)
print("quest clear total:", total, "levels:", len(qc))

rc_start = idx.get("[remove clear quest]")
rc_end = idx.get("[/remove clear quest]")
if rc_start is not None and rc_end is not None:
    ids = [c["value"] for c in cells[rc_start+1:rc_end] if c.get("type") == 0]
    print("[remove clear quest]:", ids)
else:
    print("[remove clear quest]: MISSING", rc_start, rc_end)

sq_start = idx.get("[show quest]"); sq_end = idx.get("[/show quest]")
if sq_start is not None:
    ids = [c["value"] for c in cells[sq_start+1:sq_end] if c.get("type") == 0]
    print("[show quest]:", ids)

bq_start = idx.get("[branch quest]"); bq_end = idx.get("[/branch quest]")
if bq_start is not None and bq_end is not None:
    # 分支表结构未知，打印全部 token
    print("[branch quest] tokens:")
    for c in cells[bq_start:bq_end+1]:
        print("  ", c if c.get("type") == 3 else c.get("value"))
else:
    print("[branch quest]: MISSING", bq_start, bq_end)

# 关键语义验证
qc_all = {}
for r in qc:
    qc_all.setdefault(r["level"], set()).update(r["ids"])
merged = set()
for lv, s in sorted(qc_all.items()):
    merged |= s
print("\n12911 in merged (before remove):", 12911 in merged)
print("22987 in merged:", 22987 in merged)
# 22987 是否出现在 branch quest
bq_txt = json.dumps([c.get("text", c.get("value")) for c in cells[bq_start:bq_end+1]]) if bq_start is not None else ""
print("22987 in [branch quest]:", "22987" in bq_txt or 22987 in [c.get("value") for c in cells])
# 12884 位置
where = [lv for lv, s in qc_all.items() if 12884 in s]
print("12884 appears in quest clear levels:", where)

"""只读提取赤红铁矿的四份编译规则表，保留记录与父子关系供协议核对。"""

import argparse
import hashlib
import json
from pathlib import Path
import struct
import sys

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "server/work/dfo-lan/scripts"))
from pvf_archive import Archive


def records(raw):
    version, count = struct.unpack_from("<II", raw)
    cell_end = struct.unpack_from("<I", raw, 20)[0] + 4
    column_end = struct.unpack_from("<I", raw, 28)[0] + 4
    assert version == 1 and 36 <= cell_end <= column_end < len(raw)
    # 列索引也可能包含0x5b，不能从记录末尾盲搜第一个左方括号。
    pool_at = column_end
    while pool_at < len(raw) and raw[pool_at] == 0:
        pool_at += 1
    pool = raw[pool_at:]
    assert pool.startswith(b"[")

    def text(lo, hi):
        lo, hi = sorted((lo, hi))
        assert 0 <= lo <= hi <= len(pool)
        return pool[lo:hi].decode("ascii")

    offset, result = 36, []
    for index in range(count):
        lo, hi, flags, parent, ncells, nrefs = struct.unpack_from("<QQIqQQ", raw, offset)
        assert -1 <= parent < count and ncells <= 10000 and nrefs <= count
        offset += 44
        cells, refs = [], {}
        for _ in range(ncells):
            tag = struct.unpack_from("<I", raw, offset)[0]
            offset += 4
            if tag in (4, 5):
                value = struct.unpack_from("<d", raw, offset)[0]
                value = int(value) if value.is_integer() else value
                size = 8
            elif tag == 1:
                value = text(*struct.unpack_from("<QQ", raw, offset))
                size = 16
            elif tag == 3:
                value, size = raw[offset], 1
            else:
                raise ValueError(f"尚未支持的单元类型：{tag}")
            cells.append(value)
            offset += size
        for _ in range(nrefs):
            a, b, length = struct.unpack_from("<QQQ", raw, offset)
            offset += 24
            assert length <= count
            refs[text(a, b)] = list(struct.unpack_from("<" + "Q" * length, raw, offset))
            offset += 8 * length
        result.append(dict(index=index, name=text(lo, hi), flags=flags,
                           parent=parent, cells=cells, refs=refs))
    assert offset == cell_end
    for row in result:
        for name, children in row["refs"].items():
            assert all(i < count and result[i]["parent"] == row["index"]
                       and result[i]["name"] == name for i in children)
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--rewards-output", type=Path)
    args = parser.parse_args()
    archive = Archive(ROOT / "client/Script.pvf")
    wanted = {"bleedingmine.ctp", "bleedingmineclientscript.ctp",
              "bleedingmineuiscript.ctp", "bleedingminerewardscript.ctp"}
    files = {}
    for index in range(archive.count):
        name, parent, _, _, _, kind = archive.record(index)
        if kind != 4 or archive.resolve(name).lower() not in wanted:
            continue
        path = (archive.resolve(parent) + "/" + archive.resolve(name)).lower().lstrip("/")
        if not path.startswith("contents/2025/bleedingmine/etc/"):
            continue
        archive.files[path] = index
        raw = archive.read(path)
        files[path] = dict(sha256=hashlib.sha256(raw).hexdigest(), records=records(raw))
    assert len(files) == len(wanted)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(dict(source_sha256=archive.source_sha256, files=files),
                                      ensure_ascii=False, indent=2), encoding="utf-8")
    if args.rewards_output:
        export_rewards(archive, files, args.rewards_output)
    print("四份规则表已只读提取，并通过记录边界与父子引用校验。")


def export_rewards(archive, files, output):
    """从当前原版资源递归导出矿区奖励，保留负数空奖签，禁止取绝对值。"""
    base = "contents/2025/bleedingmine/etc/"
    rules = files[base + "bleedingmine.ctp"]["records"]
    rewards = files[base + "bleedingminerewardscript.ctp"]["records"]

    def cells(rows, name):
        found = [r["cells"] for r in rows if r["name"] == name]
        assert len(found) == 1, name
        return found[0]

    stages = cells(rewards, "[dungeon clear reward]")
    assert stages[::2] == list(range(12))
    bosses = cells(rules, "[monster piece by dungeon]")
    groups = cells(rules, "[monster piece by difficulty]")
    index = json.loads((ROOT / "server/work/dfo-lan/configs/items.index.json").read_text(encoding="utf-8"))["items"]
    group_path = "etc/dungeondroptablebygroup.etc"
    if group_path not in archive.files:
        for n in range(archive.count):
            name, parent, *_ = archive.record(n)
            if archive.resolve(name).lower() == "dungeondroptablebygroup.etc" and archive.resolve(parent).lower().strip("/") == "etc":
                archive.files[group_path] = n
                break
    group_cells = archive.tokens(group_path)
    smart_groups, current, section, values = {}, None, "", []
    for tag, value in group_cells:
        if tag == 3:
            if section in ("[drop item]", "[smart drop item]") and values:
                if len(values) % 2:
                    smart_groups[current] = []
                else:
                    smart_groups.setdefault(current, []).extend(dict(template=values[n], weight=values[n+1], count=1) for n in range(0, len(values), 2))
            section, values = value, []
        elif tag == 0:
            if section == "[group]":
                current = value
            elif section in ("[drop item]", "[smart drop item]"):
                values.append(value)
    out = dict(source=hashlib.sha256(archive.data).hexdigest(), archive_sha256=archive.source_sha256, stage_boxes=stages[1::2],
               boss_boxes=dict(zip(map(str, bosses[::2]), bosses[1::2])),
               group_boxes=groups[1::2], boxes={}, items={}, combine={}, undefined_items=[])
    # 原版 name_10330673 / explain_10330673 均为 Drop Failure Dummy。
    # 它虽然在物品目录里存在，仍是图鉴碎片奖池的空奖，不能实际发放。
    out["no_drop_items"] = [10330673]
    out["combine"]["chances"] = cells(rewards, "[give bind chance]")[1::2]
    out["combine"]["maximum"] = cells(rewards, "[max bind chance]")[0]
    for family in ("equipment", "stackable"):
        parent = next(r for r in rewards if r["name"] == f"[{family} bind]")
        children = [r for r in rewards if r["parent"] == parent["index"]]
        out["combine"][family] = {r["name"].strip("[]"): r["cells"] for r in children}
    pending = set(stages[1::2] + bosses[1::2] + groups[1::2])
    for family in ("equipment", "stackable"):
        for tag, values in out["combine"][family].items():
            if tag.endswith("list"):
                pending.update(values)
    while pending:
        item = pending.pop()
        if item <= 0 or str(item) in out["items"]:
            continue
        meta = index.get(str(item))
        if not meta:
            # 索引中不存在的分支单独记录，加载端必须拒绝将其当作正常物品。
            if item not in out["undefined_items"]:
                out["undefined_items"].append(item)
            continue
        out["items"][str(item)] = meta
        if meta["kind"] != "stackable":
            continue
        tokens = archive.tokens(meta["path"])
        pools = []
        smart_group = next((tokens[i + 1][1] for i, t in enumerate(tokens) if t == (3, "[smart drop group id]")), 0)
        inside = False
        for i, (kind, value) in enumerate(tokens):
            if kind != 3:
                continue
            if value == "[rarity]":
                meta["rarity"] = tokens[i + 1][1]
            if value == "[booster info]":
                inside = True
            elif value == "[/booster info]":
                inside = False
            elif (inside and value in ("[etc]", "[stackable]", "[equipment]", "[creature]")) or (meta["stackable_type"] == "[upgradable legacy]" and value == "[int data]"):
                values = []
                for tag, number in tokens[i + 1:]:
                    if tag == 3:
                        break
                    if tag != 0:
                        raise ValueError(f"矿区奖励出现非整数奖签：{item}")
                    values.append(number)
                draws = values.pop(0) if inside else 1
                assert len(values) % 3 == 0 and 0 < draws <= 100, item
                candidates = [dict(template=values[n], weight=values[n + 1], count=values[n + 2]) for n in range(0, len(values), 3)]
                if any(490000000 <= c["template"] < 490001000 for c in candidates):
                    assert smart_group in smart_groups and smart_groups[smart_group], (item, smart_group)
                    candidates = smart_groups[smart_group]
                assert all(c["weight"] > 0 and c["count"] > 0 for c in candidates), item
                pools.append(dict(draws=draws, candidates=candidates))
                pending.update(c["template"] for c in candidates if c["template"] > 0)
        if pools:
            out["boxes"][str(item)] = pools
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(out, ensure_ascii=False, indent=2), encoding="utf-8")
    print(f"矿区奖励：12份阶段表，{len(out['boxes'])}份容器，{len(out['items'])}个物品定义。")


if __name__ == "__main__":
    main()

"""从当前客户端只读导出冒险团规则，保留来源散列，不修改 PVF。"""
import argparse
import hashlib
import json
import struct
from pathlib import Path

from pvf_archive import Archive


def section(tokens, tag):
    start = next((i + 1 for i, t in enumerate(tokens) if t == (3, tag) or t == [3, tag]), None)
    if start is None:
        return []
    end = next((i for i in range(start, len(tokens)) if tokens[i][0] == 3), len(tokens))
    return tokens[start:end]


def blocks(tokens, tag):
    opened = None
    for i, t in enumerate(tokens):
        if t[0] == 3 and t[1] == tag:
            opened = i + 1
        elif opened is not None and t[0] == 3 and t[1] == tag.replace('[', '[/'):
            yield tokens[opened:i]
            opened = None


def values(tokens, tag):
    return [struct.unpack('<f', struct.pack('<i', v))[0] if k == 2 else v for k, v in section(tokens, tag)]


def convert(info):
    exp = values(info, '[adventure exp table]')
    rules = {'max_level': values(info, '[max adventure level]')[0],
             'exp_rate': values(info, '[adventure exp rate]')[0],
             'experience': {int(exp[i]): int(exp[i+1]) for i in range(0, len(exp), 2)},
             'shops': {}}
    for block in blocks(info, '[shop]'):
        rows = values(block, '[adventurer shop purchase info]')
        assert len(rows) % 5 == 0
        rules['shops'][values(block, '[index]')[0]] = {
            'max_points': values(block, '[max shop point]')[0],
            'exp_points': values(block, '[exp to get shop point]'),
            'reset_points': any(t[1] == '[reset point]' for t in block),
            'items': [dict(zip(('template', 'level', 'price', 'limit', 'reset'), rows[i:i+5])) for i in range(0, len(rows), 5)]}
    return rules


def recommended_rules(archive):
    """按当前客户端145A28720使用的源规则导出推荐范围，不套用怪物基准等级。"""
    source = 'event/conditioneventchkdungeon.evt'
    listing = archive.tokens('list/worldmap.lst')
    worldmaps = {listing[i][1]: listing[i+1][1].lower().replace('\\', '/')
                 for i in range(0, len(listing), 2)}
    for i in range(archive.count):
        name, parent, _, _, _, kind = archive.record(i)
        if kind != 1:
            continue
        name = archive.resolve(name).lower()
        if name == 'conditioneventchkdungeon.evt' or name.endswith('.wdm'):
            path = (archive.resolve(parent)+'/'+name).lower().lstrip('/')
            archive.files[path] = i
    tokens = archive.tokens(source)

    def ranges(tag):
        cells = values(tokens, tag)
        if len(cells) % 3 or any(not isinstance(x, int) for x in cells):
            raise ValueError('推荐地下城源范围格式无效：'+tag)
        rows = {cells[i]: cells[i+1:i+3] for i in range(0, len(cells), 3)}
        if len(rows)*3 != len(cells) or any(lo < 1 or hi < lo for lo, hi in rows.values()):
            raise ValueError('推荐地下城源范围重复或倒置：'+tag)
        return rows

    direct = ranges('[specific dungeon event level]')
    world_ranges = ranges('[worldmap event level]')
    derived, ambiguous, unavailable = {}, set(), []
    sources = {source: hashlib.sha256(archive.read(source)).hexdigest()}
    for world, bounds in world_ranges.items():
        path = worldmaps.get(world)
        if path not in archive.files:
            unavailable.append(world)  # 源规则保留的旧区域，当前worldmap.lst已不登记。
            continue
        sources[path] = hashlib.sha256(archive.read(path)).hexdigest()
        for rows in blocks(archive.tokens(path), '[dungeon]'):
            at = 0
            while at < len(rows):
                if rows[at][0] != 0 or rows[at][1] <= 0:
                    raise ValueError('选图副本编号无效：'+path)
                dungeon = rows[at][1]
                at += 1
                if at < len(rows) and rows[at][0] == 3:
                    if rows[at][1] not in ('[in progress]', '[is clear quest]'):
                        raise ValueError('选图条件尚未核实：'+path)
                    at += 1
                if at >= len(rows) or rows[at][0] != 0:
                    raise ValueError('选图任务条件缺失：'+path)
                at += 1  # 每个副本后的任务编号不是另一个副本。
                if dungeon in derived and derived[dungeon] != bounds:
                    ambiguous.add(dungeon)
                derived[dungeon] = bounds
    # 专属范围优先于区域；同一旧副本在不同区域有冲突时不能任选一份。
    for dungeon in ambiguous:
        derived.pop(dungeon, None)
    derived.update(direct)
    excluded = values(tokens, '[unable dungeon]')
    return {'minimum_level': values(tokens, '[apply level]')[0],
            'excluded': sorted(set(excluded)), 'ranges': dict(sorted(derived.items())),
            'ambiguous_dungeons': sorted(ambiguous-set(direct)-set(excluded)),
            'unavailable_worldmaps': sorted(unavailable), 'sources': sources,
            'pvf_sha256': archive.source_sha256}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('pvf', type=Path)
    parser.add_argument('output', type=Path)
    parser.add_argument('--recommended', action='store_true', help='只导出推荐地下城计数规则')
    args = parser.parse_args()
    archive = Archive(args.pvf)
    if args.recommended:
        rules = recommended_rules(archive)
        args.output.write_text(json.dumps(rules, ensure_ascii=False, indent=2)+'\n', encoding='utf-8', newline='\n')
        print('已导出推荐地下城规则：', len(rules['ranges']), '个副本范围，', len(rules['excluded']), '个排除副本')
        return
    paths = ('etc/adventurersystem/adventurersystem2018.etc',)
    for i in range(archive.count):
        name, parent, _, _, _, kind = archive.record(i)
        if kind == 1 and archive.resolve(name).lower().endswith('.etc'):
            path = (archive.resolve(parent)+'/'+archive.resolve(name)).lower().lstrip('/')
            if path in paths:
                archive.files[path] = i
    rules = convert(*(archive.tokens(p) for p in paths))
    rules['sources'] = {p: hashlib.sha256(archive.read(p)).hexdigest() for p in paths}
    rules['pvf_sha256'] = archive.source_sha256
    listing = archive.tokens('list/stackable.lst')
    item_paths = {listing[i][1]: listing[i+1][1].lower() for i in range(0, len(listing), 2)}
    rules['items'] = {}
    for shop in rules['shops'].values():
        for row in shop['items']:
            identifier = row['template']
            path = item_paths[identifier]
            if not path.startswith('stackable/'):
                path = 'stackable/'+path
            tokens = archive.tokens(path)
            rules['items'][identifier] = {'path': path, 'sha256': hashlib.sha256(archive.read(path)).hexdigest(),
                'type': values(tokens, '[stackable type]')[0], 'limit': (values(tokens, '[stack limit]') or [0])[0],
                'usage_period': values(tokens, '[usable period]')}
    args.output.write_text(json.dumps(rules, ensure_ascii=False, indent=2)+'\n', encoding='utf-8', newline='\n')
    print('已导出当前冒险团等级及', len(rules['shops']), '类商店规则，不包含隐藏的传统远征')
    # 用源文本核对限购周期及货币含义，不把标签名称当规则。
    domains = archive.tokens('list/n_string.lst')
    strings = {domains[i][1]: domains[i+1][1].lower() for i in range(0, len(domains), 2)}
    for line in archive.read(strings[4]).decode('utf-16-le', errors='replace').splitlines():
        if 'adv_shop_index' in line or 'AdventurerExpedition' in line and ('reward' in line.lower() or 'coin' in line.lower()):
            print(line)


if __name__ == '__main__':
    main()

"""只读导出当前 PVF 的迷雾誓约规则及奖励定义。"""
import argparse
import hashlib
import json
import re
import struct
from pathlib import Path

from pvf_archive import Archive
from export_adventure_rules import values
from pvf_rule_fields import read_reward_ctp


def block(text, name):
    return re.findall(r'\[' + re.escape(name) + r'\](.*?)\[/' + re.escape(name) + r'\]', text, re.S)


def number(text, name):
    match = re.search(r'\[' + re.escape(name) + r'\]\s*(-?\d+)', text)
    if not match:
        raise ValueError('源标签缺失：' + name)
    return int(match[1])


def convert(text):
    result = {'season': number(text, 'season id'), 'minimum_level': number(text, 'require minimum level'),
              'oath_cost_key': number(text, 'oath cost key'), 'max_acquisitions': number(text, 'max oath acquisition count'),
              'levels': [], 'penalties': [], 'contents': [], 'rewards': []}
    for row in block(block(text, 'exp level chart')[0], 'level'):
        level = int(row.split()[0])
        result['levels'].append({'level': level, 'upper': number(row, 'acc exp'), 'fame': number(row, 'add fame value')})
        if '[display max level]' in row:
            result['display_max_level'] = level
    for group in block(block(text, 'penalty rule set')[0], 'level'):
        low, high = map(int, group.split()[:2])
        for row in block(group, 'rule'):
            cells = list(map(int, re.findall(r'-?\d+', block(row, 'exp ratio')[0])))
            assert len(cells) % 3 == 0
            result['penalties'].append({'minimum_level': low, 'maximum_level': high, 'rule': number(row, 'no'),
                'default_ratio': number(row, 'default exp ratio'), 'ranges': [cells[i:i+3] for i in range(0, len(cells), 3)]})
    # 类型值来自当前客户端 0x1476A9EA0，不依赖文本排列顺序。
    types = {'normal dungeon': 1, 'higher dungeon': 2, 'legion': 3, 'raid': 4, 'etc': 5, 'oath': 6}
    for row in block(text, 'dungeon category'):
        name = re.search(r'\[type\]\s*`([^`]+)`', row)[1]
        exp = list(map(int, re.findall(r'-?\d+', block(row, 'default exp')[0])))
        assert len(exp) == 1 or exp[0] == -1 and len(exp) % 2 == 1
        ids = block(row, 'dungeon index') or block(row, 'raid area')
        for identifier in re.findall(r'^\s*(\d+)\s+', ids[0], re.M):
            result['contents'].append({'id': int(identifier), 'category': types[name],
                'default_exp': exp[0], 'difficulties': dict(zip(exp[1::2], exp[2::2])),
                'rule': number(row, 'enable penalty rule set'), 'cs_only': '[cs only]' in row})
    for row in block(block(text, 'mist oath level reward')[0], 'level'):
        template, count = map(int, re.search(r'\[reward\]\s*(\d+)\s+(\d+)', row).groups())
        result['rewards'].append({'level': int(row.split()[0]), 'mask': number(row, 'bit mask'), 'template': template, 'count': count})
    result['oath_equipment'] = list(map(int, re.findall(r'\d+', block(text, 'oath equipment list')[0])))
    result['special_reward'] = list(re.search(r'\[30lv special reward\]\s*`([^`]+)`\s*`([^`]+)`\s*(\d+)', text).groups())
    assert len(result['levels']) == 120 and result['display_max_level'] == 100 and len(result['rewards']) == 3
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('pvf', type=Path)
    parser.add_argument('output', type=Path)
    parser.add_argument('--inspect-cost', action='store_true')
    args = parser.parse_args()
    archive = Archive(args.pvf)
    source = 'contents/system/seasonlevel/main.cos'
    for i in range(archive.count):
        name, parent, _, _, _, kind = archive.record(i)
        if kind not in (1, 3, 4):
            continue
        name = archive.resolve(name).lower()
        if name in ('main.cos', 'costs.ctp'):
            path = (archive.resolve(parent) + '/' + name).lower().lstrip('/')
            archive.files[path] = i
    raw = archive.read(source)
    text = raw.decode('utf-16-le') if raw.startswith(b'\xff\xfe') or b'\x00' in raw[:10] else raw.decode('utf8')
    result = convert(text)
    result['source'] = source
    result['sha256'] = hashlib.sha256(raw).hexdigest()
    result['pvf_sha256'] = archive.source_sha256
    raw_cost = archive.read('etc/costs.ctp')
    costs = read_reward_ctp(raw_cost, allow_cost_values=True)
    for row in costs:
        if row['name'] != '[cost]':
            continue
        fields = {tag: costs[children[0]]['cells'] for tag, children in row['refs'].items()}
        if fields.get('[idx]') == [result['oath_cost_key']]:
            def subtree(index):
                record = costs[index]
                return {'cells': record['cells'], **{tag: [subtree(child) for child in children] for tag, children in record['refs'].items()}}
            tree = subtree(row['index'])
            print('誓约费用原文：', tree)
            result['oath_cost_source'] = tree
            required = tree['[required item]'][0]
            assert '[optional]' not in tree
            cells = required['[materials]'][0]['cells']
            assert len(cells) % 2 == 0 and all(isinstance(v, int) and v > 0 for v in cells)
            result['oath_cost'] = {'gold': max(0, required['[gold]'][0]['cells'][0]),
                'materials': [{'template': cells[i], 'count': cells[i+1]} for i in range(0, len(cells), 2)]}
    assert 'oath_cost' in result
    result['cost_sha256'] = hashlib.sha256(raw_cost).hexdigest()
    if args.inspect_cost:
        return
    listing = archive.tokens('list/stackable.lst')
    paths = {listing[i][1]: listing[i+1][1].lower() for i in range(0, len(listing), 2)}
    result['items'] = {}
    for identifier in [r['template'] for r in result['rewards']] + [int(result['special_reward'][2])]:
        path = paths[identifier]
        if not path.startswith('stackable/'):
            path = 'stackable/' + path
        tokens = archive.tokens(path)
        result['items'][identifier] = {'path': path, 'sha256': hashlib.sha256(archive.read(path)).hexdigest(),
            'type': values(tokens, '[stackable type]')[0], 'limit': (values(tokens, '[stack limit]') or [0])[0],
            'usage_period': values(tokens, '[usable period]')}
    result['capsules'] = {}
    tag = '[season level system exp]'
    at = archive.pools[0].find((tag+'\x00').encode())
    if at >= 0:
        offset = at*2
    else:
        at = archive.pools[1].find((tag+'\x00').encode('utf-16-le'))
        assert at >= 0 and at % 2 == 0
        offset = at | 1
    needle = struct.pack('<Bi', 6, offset)
    # 按压缩块扫描，避免每个物品重解压同一大型资源块。
    stack_paths = [(identifier, path if path.startswith('stackable/') else 'stackable/'+path) for identifier, path in paths.items()]
    for identifier, path in sorted(stack_paths, key=lambda row: archive.record(archive.files[row[1]])[2]):
        raw_item = archive.read(path)
        if needle not in raw_item:
            continue
        tokens = archive.tokens(path)
        action = values(tokens, '[action type]')
        if not action or action[0] != '[season level system exp]':
            continue
        assert len(action) == 4, (identifier, action)
        result['capsules'][identifier] = {'category': action[1], 'id': action[2], 'difficulty': action[3],
            'path': path, 'sha256': hashlib.sha256(raw_item).hexdigest(), 'minimum_level': (values(tokens, '[minimum level]') or [1])[0]}
    print('经验道具：', len(result['capsules']))
    args.output.write_text(json.dumps(result, ensure_ascii=False, indent=2) + '\n', encoding='utf8', newline='\n')
    print('已导出', len(result['levels']), '个阶段、', len(result['contents']), '个玩法规则、', len(result['rewards']), '个阶段奖励')
    domains = archive.tokens('list/n_string.lst')
    strings = {domains[i][1]: domains[i+1][1].lower() for i in range(0, len(domains), 2)}
    for domain in (4, 20):
        for line in archive.read(strings[domain]).decode('utf-16-le', errors='replace').splitlines():
            if 'season_level_' in line.lower():
                print(line)


if __name__ == '__main__':
    main()

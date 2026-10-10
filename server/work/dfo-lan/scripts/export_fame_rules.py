"""只读导出当前客户端名望规则，保留来源与段落顺序。"""

import argparse
import hashlib
import json
import re
import struct
from pathlib import Path

from reinforcement_pvf_reader import Archive


def sections(tokens):
    result = []
    for kind, value in tokens:
        if kind == 3:
            result.append({"tag": value, "values": []})
        elif result:
            if kind == 2:
                value = struct.unpack('<f', struct.pack('<i', value))[0]
            result[-1]["values"].append(value)
    return result


def field(rows, tag):
    return next((s['values'] for s in reversed(rows) if s['tag'] == tag), [])


def blocks(rows, tag):
    begin = None
    for i, row in enumerate(rows):
        if row['tag'] == tag:
            begin = i + 1
        elif row['tag'] == tag.replace('[', '[/') and begin is not None:
            yield rows[begin:i]
            begin = None


def pairs(values):
    assert len(values) % 2 == 0
    return {int(values[i]): int(values[i+1]) for i in range(0, len(values), 2)}


def compile_rules(archive, raw):
    rows = raw['sections']
    result = {'version': 1, 'source': raw['source'], 'sources': raw['sources'],
              'tables': {}, 'refine': {}, 'refine_115': {}, 'items': {},
              'sets': {}, 'item_points': {}, 'expanded': [],
              'awakening': {}, 'memory_restore': {}, 'memory_activate': {},
              'memory_ruminations': {}, 'sole_quality': {}, 'sole_penalty': {}}
    for b in blocks(rows, '[info]'):
        result['tables'][field(b, '[type]')[0]] = pairs(field(b, '[table]'))
    for b in blocks(rows, '[correspond list]'):
        if field(b, '[type]') == ['separate upgrade']:
            assert field(b, '[correspond type]') == ['normal upgrade']
            result['refine'] = pairs(field(b, '[formula]'))
            conditional = field(b, '[level condition formula]')
            assert conditional[:2] == [115, 1000]
            result['refine_115'] = pairs(conditional[2:])
    for point, fame in pairs(field(rows, '[add value by expand set point]')).items():
        result['expanded'].append({'point': point, 'fame': fame})
    for name in ('memory_restore', 'memory_activate', 'memory_ruminations'):
        result[name] = pairs(field(rows, '[' + name.replace('_', ' ') + ' fame value]'))
    awakening = {}
    values = field(rows, '[equipment awakening fame value]')
    assert len(values) % 3 == 0
    for i in range(0, len(values), 3):
        group, rank, fame = values[i:i+3]
        awakening.setdefault(group, {})[rank] = fame
    for name, tag in (('sole_quality', '[sole equipment quality fame value]'),
                      ('sole_penalty', '[sole equipment penalty fame value]')):
        values = field(rows, tag)
        assert len(values) % 3 == 0
        for i in range(0, len(values), 3):
            template, point, fame = values[i:i+3]
            result[name].setdefault(template, {})[point] = fame

    source = 'etc/115lvability/setpointinfo.cos'
    text = raw['point_source_text']
    result['sources'][source] = hashlib.sha256(archive.read(source)).hexdigest()
    groups = {}
    for body in re.findall(r'\[info\](.*?)\[/info\]', text.split('[/set point]')[0], re.S):
        v = {k: int(n) for k, n in re.findall(r'\[([^\]]+)\]\s*(-?\d+)', body)}
        groups.setdefault(v['group'], []).append({
            'set': v['part set index'], 'point': v['value'], 'awakening': v['awakening']})
    grouping = 'etc/equipmentgrouping.etc'
    result['sources'][grouping] = hashlib.sha256(archive.read(grouping)).hexdigest()
    for b in blocks(raw['group_sections'], '[ability group]'):
        group = field(b, '[index]')[0]
        for template in field(b, '[list]'):
            for rank, fame in awakening.get(group, {}).items():
                # 原生14739AF20：同一装备属于多个分组时取最高值，不累加。
                entry = result['awakening'].setdefault(template, {})
                entry[rank] = max(entry.get(rank, 0), fame)
        for point in groups.get(group, []):
            for template in field(b, '[list]'):
                entries = result['item_points'].setdefault(template, [])
                if point not in entries:
                    assert not any(p['set'] == point['set'] and p['awakening'] == point['awakening']
                                   for p in entries), ('套装分数歧义', template)
                    entries.append(point)

    listing = raw['set_sections']['etc/115lvability/equipmentsetpointtable.lst']
    point_paths = {listing[i][1]: 'etc/115lvability/' + listing[i+1][1].lower()
                   for i in range(0, len(listing), 2)}
    for path in point_paths.values():
        assert path in archive.files, path
    point_values = {}
    for i, path in point_paths.items():
        point_values[i] = field(sections(archive.tokens(path)), '[fame value]')
        result['sources'][path] = hashlib.sha256(archive.read(path)).hexdigest()
    for b in raw['set_sections']['etc/equipmentpartset.etc']:
        if b['tag'] != '[equipment part set]' or len(b['values']) < 2:
            continue
        set_id, relative = b['values'][:2]
        path = 'equipment/' + relative.lower()
        if path not in archive.files:
            continue
        set_rows = sections(archive.tokens(path))
        entries = []
        for i, s in enumerate(set_rows):
            if s['tag'] != '[piece set point]':
                continue
            threshold = s['values'][0]
            # 限定当前段，不能跨到下一档读取table。
            end = next((j for j in range(i+1, len(set_rows))
                        if set_rows[j]['tag'] in ('[/piece set point]', '[piece set point]')), len(set_rows))
            table = field(set_rows[i+1:end], '[set point table index]')
            if table and point_values.get(table[0]):
                entries.append({'point': threshold, 'fame': point_values[table[0]][0]})
        if entries:
            result['sets'][set_id] = sorted(entries, key=lambda v: v['point'])
            result['sources'][path] = hashlib.sha256(archive.read(path)).hexdigest()

    # 按实际数据块顺序扫描所有堆叠物，避免只按名称筛掉数字编号的附魔卡。
    index = json.loads((Path(__file__).resolve().parent.parent / 'configs/items.index.json').read_bytes())['items']
    stackables = []
    for template, item in index.items():
        path = item.get('path', '').lower().lstrip('/')
        if item.get('kind') == 'stackable' and path in archive.files:
            entry = archive.record(archive.files[path])
            stackables.append((entry[2], entry[3], int(template), path))
    for _, _, template, path in sorted(stackables):
        values = sections(archive.tokens(path))
        direct, table = field(values, '[fame value]'), field(values, '[fame table]')
        additional = field(values, '[add fame value]')
        if direct or table or additional:
            value = {'value': direct[0] if direct else 0}
            if additional:
                value['additional'] = additional[0]
            if table:
                assert len(table) == 2
                value.update(table=table[0], index=table[1])
            result['items'][template] = value
            result['sources'][path] = hashlib.sha256(archive.read(path)).hexdigest()
    print('名望表、套装、附加物品：', len(result['tables']), len(result['sets']), len(result['items']), flush=True)
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--source', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--raw', action='store_true', help='仅用于离线分析的原始段落导出')
    args = parser.parse_args()
    archive = Archive(args.source)
    source = 'etc/famevalueinfo.etc'
    candidates = []
    for i in range(archive.count):
        name, folder, _, _, _, kind = archive.record(i)
        if kind not in (1, 3, 4):
            continue
        name = archive.resolve(name).lower()
        folder = archive.resolve(folder).lower()
        if name not in ('famevalueinfo.etc', 'setpointinfo.cos', 'equipmentgrouping.etc') and not folder.startswith('etc/115lvability/equipmentsetpointtable') and not ('set' in name and name.endswith(('.etc', '.lst', '.ctp'))):
            continue
        path = (folder + '/' + name).lower().lstrip('/')
        archive.files[path] = i
        if path != source:
            candidates.append(path)
    raw = archive.read(source)
    result = {'version': 1, 'source': archive.source_sha256,
              'sources': {source: hashlib.sha256(raw).hexdigest()},
              'sections': sections(archive.tokens(source)), 'set_sources': candidates}
    result['set_sections'] = {}
    for path in ('etc/equipmentpartset.etc', 'etc/115lvability/equipmentsetpointtable.lst'):
        raw = archive.read(path)
        result['sources'][path] = hashlib.sha256(raw).hexdigest()
        result['set_sections'][path] = sections(archive.tokens(path)) if path.endswith('.etc') else archive.tokens(path)
    point_path = 'etc/115lvability/setpointinfo.cos'
    result['point_source_text'] = archive.read(point_path).decode('utf-16-le')
    result['group_sections'] = sections(archive.tokens('etc/equipmentgrouping.etc'))
    if not args.raw:
        result = compile_rules(archive, result)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(result, ensure_ascii=False, indent=2) + '\n', encoding='utf-8', newline='\n')
    print('已导出名望规则：', args.output)


if __name__ == '__main__':
    main()

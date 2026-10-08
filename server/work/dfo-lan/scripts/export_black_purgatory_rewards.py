"""导出黑鸦PVF奖励范围，独立装备概率采用用户确认的本服配置。"""
import argparse
import hashlib
import json
from pathlib import Path
import re

from pvf_archive import Archive
from pvf_rule_fields import fields


DUNGEON = 100000527
CARD_SCRIPT = 'etc/dungeonspecialreward.etc'
GROUP_SCRIPT = 'etc/itemdictionary/customroutingwaygroup.cos'
ROUTE_SCRIPT = 'etc/itemdictionary/customroutingway.etc'
GROUPS = {1010001: ('epic', 4), 1010007: ('corrupt_product', 4),
          1010500: ('mythic', 7)}


def register_sources(archive):
    wanted = {CARD_SCRIPT, GROUP_SCRIPT, ROUTE_SCRIPT}
    if wanted <= archive.files.keys():
        return
    filenames = {Path(path).name for path in wanted}
    for number in range(archive.count):
        name, parent, _, _, _, kind = archive.record(number)
        if kind not in (1, 3):
            continue
        filename = archive.resolve(name).lower()
        if filename not in filenames:
            continue
        path = (archive.resolve(parent) + '/' + filename).lower().lstrip('/')
        if path in wanted:
            archive.files[path] = number
    if wanted - archive.files.keys():
        raise ValueError('缺少黑鸦奖励源文件')


def card_candidates(archive):
    cells = archive.tokens(CARD_SCRIPT)
    sections = []
    for at, token in enumerate(cells[:-1]):
        if token != (3, '[dungeon]') or cells[at + 1] != (0, DUNGEON):
            continue
        end = cells.index((3, '[/dungeon]'), at)
        section = cells[at + 1:end]
        if len(section) >= 4 and section[:4] == [(0, DUNGEON), (0, 0), (0, 1), (0, 1)]:
            sections.append(section)
    if len(sections) != 1:
        raise ValueError('小队翻牌段不唯一或结构已变化')
    cells = sections[0][4:]
    if len(cells) % 8 or any(typ != 0 for typ, _ in cells):
        raise ValueError('小队翻牌记录不符合当前客户端的八列结构')
    ordinary = {condition: [] for condition in range(5)}
    vip = {condition: [] for condition in range(5)}
    for at in range(0, len(cells), 8):
        item, kind, reserved, condition_type, condition, weight, style, count = [
            value for _, value in cells[at:at + 8]]
        if (kind not in (0, 1) or reserved != -1 or condition_type != 2 or
                condition not in ordinary or not 0 < weight <= 10000 or
                count != 1 or not 0 <= style <= 3):
            raise ValueError('翻牌条件、权重或数量尚未核实')
        (ordinary if kind == 0 else vip)[condition].append(
            {'Template': item, 'Weight': weight, 'Count': count})
    for table in (ordinary, vip):
        if any(rows != table[0] for rows in table.values()):
            raise ValueError('不同条件的翻牌表不再相同，不能合并使用')
        if sum(row['Weight'] for row in table[0]) != 10000:
            raise ValueError('源翻牌权重之和不是10000')
    if len(ordinary[0]) != 5 or len(vip[0]) != 1:
        raise ValueError('源翻牌分支数量发生变化')
    return ordinary[0], vip[0]


def equipment_candidates(archive, index):
    text = archive.read(GROUP_SCRIPT).decode('utf-16-le')
    result = {}
    for section in re.findall(r'\[group info\](.*?)\[/group info\]', text, re.S):
        match = re.search(r'\[id\]\s*(\d+)', section)
        if not match or int(match[1]) not in GROUPS:
            continue
        group = int(match[1])
        kind, rarity = GROUPS[group]
        target = re.search(r'\[target\](.*?)\[/target\]', section, re.S)
        if target is None or kind in result:
            raise ValueError('装备来源分组缺失或重复')
        ids = [int(value) for value in target[1].split()]
        if not ids or len(ids) != len(set(ids)):
            raise ValueError('装备来源分组为空或物品重复')
        entries = []
        for identifier in ids:
            item = index['items'].get(str(identifier))
            if not item or item['kind'] != 'equipment':
                raise ValueError('装备未导入物品目录：' + str(identifier))
            path = item['path'].lower()
            definition = fields(archive.tokens(path))
            if definition.get('[rarity]') != [rarity]:
                raise ValueError('源装备品质不符：' + str(identifier))
            level = definition.get('[minimum level]', [])
            if len(level) != 1 or not isinstance(level[0], int) or level[0] <= 0:
                raise ValueError('源装备等级无效：' + str(identifier))
            # 按来源组收集，不能用旧攻略中的100级过滤：当前神话已是105级。
            entries.append({'template': identifier, 'minimum_level': level[0],
                            'rarity': rarity, 'path': path,
                            'sha256': hashlib.sha256(archive.read(path)).hexdigest()})
        result[kind] = {'source_group': group, 'candidates': entries}
    if len(result) != len(GROUPS):
        raise ValueError('黑鸦装备来源组不完整')
    return result


def export_config(archive, configs, local_rates=(100000, 1000, 10000)):
    if len(local_rates) != 3 or any(not 0 <= rate <= 1000000 for rate in local_rates):
        raise ValueError('本服独立掉率必须为百万分之0至1000000')
    register_sources(archive)
    index = json.loads((configs / 'items.index.json').read_text(encoding='utf-8'))
    cards, vip = card_candidates(archive)
    return {
        'model': 'black-purgatory-card-v1',
        'source': index['source']['checksum'],
        'client_pvf_sha256': archive.source_sha256,
        'script': CARD_SCRIPT,
        'script_sha256': hashlib.sha256(archive.read(CARD_SCRIPT)).hexdigest(),
        'dungeon': DUNGEON,
        'cards': cards,
        'vip_source_only': vip,
        'boss_equipment': {
            'model': 'local-black-purgatory-boss-v1',
            'status': '装备范围来自PVF；概率为2026-09-29用户确认的本服暂定值，不是官方爆率',
            'denominator': 1000000,
            'rates': dict(zip(('epic', 'mythic', 'corrupt_product'), local_rates)),
            'selection': '最终领主每次分别抽取，各组内部等概率；同次挑战只生成一次',
            'group_script': GROUP_SCRIPT,
            'group_script_sha256': hashlib.sha256(archive.read(GROUP_SCRIPT)).hexdigest(),
            'routing_script': ROUTE_SCRIPT,
            'routing_script_sha256': hashlib.sha256(archive.read(ROUTE_SCRIPT)).hexdigest(),
            'groups': equipment_candidates(archive, index),
        },
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--pvf', type=Path, required=True)
    parser.add_argument('--client-root', type=Path)
    parser.add_argument('--configs', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--local-boss-rates', type=int, nargs=3,
                        default=(100000, 1000, 10000),
                        help='本服史诗、神话、腐蚀产物概率，分母1000000')
    args = parser.parse_args()
    archive = Archive(args.pvf, args.client_root)
    result = export_config(archive, args.configs, args.local_boss_rates)
    args.output.write_text(json.dumps(result, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
    print('已导出黑鸦翻牌5个分支；装备来源清单：' +
          '、'.join(str(len(group['candidates'])) for group in result['boss_equipment']['groups'].values()))


if __name__ == '__main__':
    main()

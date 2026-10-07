"""只读提取「增幅」（Amplification，NPC Klonter）的服务端规则，生成 configs/amplify-upgrade.json。

数据分两半（与金币强化同一套路）：

1) 客户端 PVF `etc/amplifyupgrade.etc`（`[table type] = amplify` 的 24 值/级矩阵）——
   材料模板、每级材料数量、金币列，以及上限/破坏等级/总伤害加成等段；
2) 官方页的成功率与失败惩罚（2026-09-27 由服务器主人提供）。客户端 PVF 里
   **不存在**任何成功率表（强化已用「枚举全部段名 + 全库数值扫描 + 对比单机补丁
   PVF」三处一致证实），所以这部分只能以官方数据为准，标注 source 便于日后替换。

    python scripts/export_amplify_upgrade.py \
        --source D:/115us/client/Script.pvf --client-root D:/115us/client \
        --output configs/amplify-upgrade.json
"""
import argparse
import json
from pathlib import Path

from reinforcement_pvf_reader import Archive

TABLE_PATH = 'etc/amplifyupgrade.etc'
TABLE_TYPE = 'amplify'
MATRIX_WIDTH = 24          # 与 upgrade.etc 同族；实机锚点：+0 → 材料 1，+9 → 材料 10
MATERIAL_FIELD = 14        # 0 基：第 14 个 = 材料模板（3242 矛盾结晶体）
MATERIAL_COUNT_FIELD = 15  # 0 基：第 15 个 = 材料数量（= 当前等级 + 1）
GOLD_FIELDS = (8, 9)       # 0 基：金币列（+0..+3 为 0，+4 起才有费用）

# 官方页数据（2026-09-27 由服务器主人提供）。PVF 无成功率表，以此为准。
OFFICIAL = {
    'source': 'official page 2026-09-27 (amplify: NPC Klonter, material Crystallized Chaos)',
    'success_rate_percent_by_level': {
        # 从等级 L 增幅到 L+1 的成功率
        '0': 100, '1': 100, '2': 100, '3': 100,
        '4': 70, '5': 60, '6': 50, '7': 50, '8': 40, '9': 30,
        'default': 30,
    },
    'failure_penalty': {
        # 失败时的处理；'attempt_from' 指「当前等级」
        '0-6': 'none',              # +0..+6 失败无变化（只消耗材料与金币）
        '7-9': 'level_down_1',      # 尝试 7→8 / 8→9 / 9→10 失败降一级
        '10+': 'destroy',           # +10 及以上失败摧毁；保护券则不碎但归零
    },
    'failure_bonus_percent_point': {
        # 失败补正：连续失败每次累加，成功或成功后清零
        '4-6': 10,
        '7-9': 5,
    },
    'protection_ticket': '不碎，但增幅等级归零',
    'safe_amplify': {
        'source': '官方页 https://www.dfoneople.com/gameinfo/guide/Advanced-Game-Information/Equipment-System/Reinforce,-Amplify,-Refine （Safe Amplification 表，2025-03-18 更新）。'
                  'PVF [safe upgrade] 与官方表逐格吻合（+0→1 武器 x36/430,100；+9→10 武器 x320/5,084,870）。',
        'material': 'Harmonious Crystals（模板见 safe_material_templates，PVF 段 [safe upgrade]）',
        # ★ 官方条件原文："+9 or lower Amplified" —— 即**当前 +9 时仍可安全增幅到 +10**。
        #   所以可行区间是「当前等级 <= +9」，不是「目标等级 < +9」。
        #   早期实现误写成「当前 >= +9 就拒绝」，正好把官方允许的 +9→+10 这一档挡掉了。
        'condition': '装备等级 >= 100 且品质在 safe_upgrade_usable_rarity 内（Unique 或更稀有），且当前增幅 <= +9',
        'max_level': 9,       # 含义：**当前等级**上限（+9 → +10 仍走安全路径）
        'max_reachable': 10,  # 安全增幅能到的最高等级
        'cost': '材料数量与金币取 [safe upgrade] 中该等级对应的一行（武器/非武器不同）；金币在第 8 列',
        # 安全增幅与普通增幅共用同一套成功率 —— 「安全」体现在失败无惩罚（不掉级不碎），
        # 代价是材料贵得多（对比：普通 +0 只要 1 个矛盾结晶体，安全 +0 要 36 个）。
        'success_rate': '与普通增幅同一张表',
        'failure_penalty': 'none（不掉级、不摧毁）',
        'failure_bonus_percent_point': {'4-6': 10, '7-9': 5},
        'note': '失败补正（连续失败累加成功率）本次未实现，留待实机确认后补齐',
    },
}

# 增幅等级写在装备实例行的哪个偏移：与「增幅书打红字」共用同一处 ——
# offset 19 = 次元属性类型（1..4），offset 20 = 次元属性数值 = 增幅等级 +N。
# 前置条件：装备必须已有次元属性（先用增幅书 CMD205 打红字），否则不能增幅。
AMPLIFY_TYPE_OFFSET = 19
AMPLIFY_VALUE_OFFSET = 20
# ★ 更正（2026-09-28 实机 + 存档取证）：增幅等级**不是** offset 20。
# offset 20 是次元属性的数值（红字），offset 10 的低五位才是由强化与增幅共用的等级字节
# （佐证：黄金增幅书会「删除当前强化等级」，清的就是 offset 10 低五位）。
AMPLIFY_LEVEL_OFFSET = 10


def load_tokens(archive, path):
    for index in range(archive.count):
        name, folder, _, _, _, kind = archive.record(index)
        if kind not in (1, 3):
            continue
        full = (archive.resolve(folder) + '/' + archive.resolve(name)).lower().lstrip('/')
        if full == path:
            archive.files[path] = index
            return archive.tokens(path)
    raise SystemExit('PVF 中找不到 ' + path)


def value_of(token):
    typ, val = token
    if typ == 2:
        import struct
        return struct.unpack('<f', struct.pack('<I', val & 0xffffffff))[0]
    return val


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--source', type=Path, required=True)
    parser.add_argument('--client-root', type=Path, default=None)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()

    archive = Archive(args.source, client_root=args.client_root)
    tokens = load_tokens(archive, TABLE_PATH)

    # [table] 矩阵
    tables, current = [], None
    for typ, val in tokens:
        if typ == 3:
            if val == '[table]':
                current = []
                tables.append(current)
            elif val == '[/table]':
                current = None
            continue
        if current is not None:
            current.append((typ, val))

    matrix = None
    for block in tables:
        if block and block[0][0] in (6, 8) and block[0][1] == TABLE_TYPE:
            matrix = block[1:]
            break
    if matrix is None:
        raise SystemExit('PVF 中找不到 [table type] = ' + TABLE_TYPE)
    if len(matrix) % MATRIX_WIDTH:
        raise SystemExit('amplify 矩阵长度不是 %d 的整数倍' % MATRIX_WIDTH)

    levels = []
    for i in range(len(matrix) // MATRIX_WIDTH):
        row = [value_of(x) for x in matrix[i * MATRIX_WIDTH:(i + 1) * MATRIX_WIDTH]]
        levels.append({
            'level': i,
            'material_template': row[MATERIAL_FIELD],
            'material_count': row[MATERIAL_COUNT_FIELD],
            'gold_columns': [row[f] for f in GOLD_FIELDS],
        })
    templates = sorted({lv['material_template'] for lv in levels})
    if len(templates) != 1:
        raise SystemExit('材料模板不唯一: %s' % templates)

    # 其余段
    sections, name = {}, None
    for typ, val in tokens:
        if typ == 3:
            name = None if val.startswith('[/') else val
            if name:
                sections.setdefault(name, [])
            continue
        if name is not None:
            sections[name].append(value_of((typ, val)))

    def values(section):
        return [v for v in sections.get(section, [])]

    # 安全增幅：段结构与强化一致（8 列/级），且**每级两行**（武器 / 非武器消耗不同）。
    safe_raw = values('[safe upgrade]')
    safe_rows = []
    for i in range(0, len(safe_raw) - 7, 8):
        row = safe_raw[i:i + 8]
        # ★ 列语义经官方页校准（dfoneople Reinforce/Amplify/Refine “Safe Amplification” 表）：
        #   +0→1 武器 = Harmonious Crystal x36 / 430,100 Gold；+9→10 武器 = x320 / 5,084,870 Gold。
        #   这两组数字都落在第 8 列（row[7]），所以 **金币取 row[7]**；
        #   row[2]（30000/40000/...）是另一列（量级像普通增幅的金币），**不是**安全增幅费用。
        safe_rows.append({'level': row[0], 'enabled': row[1], 'gold': row[7],
                          'gold_column_raw': row[2], 'b': row[3],
                          'c': row[4], 'material': row[5], 'count': row[6],
                          'value': row[7]})

    # [safe upgrade item condition] 是嵌套段（[level] 100 + [usable rarity] 六个品质），
    # 顶层同名 [level] 会互相覆盖，所以单独扫一遍。
    safe_condition = {}
    inside, child = False, None
    for typ, val in tokens:
        if typ == 3:
            if val == '[safe upgrade item condition]':
                inside, child = True, None
                continue
            if val == '[/safe upgrade item condition]':
                inside = False
                continue
            if inside:
                child = val
                safe_condition.setdefault(child, [])
            continue
        if inside and child is not None:
            safe_condition[child].append(value_of((typ, val)))

    safe_materials = sorted({r['material'] for r in safe_rows})

    out = {
        'version': 1,
        'source': archive.source_sha256,
        'table': TABLE_PATH,
        'table_type': TABLE_TYPE,
        'matrix_width': MATRIX_WIDTH,
        'material_template': templates[0],
        'materials': [
            {'template': 3242, 'path': 'stackable/material/crystal_contradictory.stk',
             'name': 'Crystallized Chaos / 矛盾结晶体'},
        ],
        'safe_material_templates': safe_materials,
        'safe_materials': [
            {'template': 10327282, 'path': '(Harmonious Crystals / 115 副本产出)',
             'name': 'Harmonious Crystals'},
        ],
        'levels': levels,
        'gold_semantics': '金币取 gold_columns[0]（+0..+3 为 0，+12 起封顶 80000）；'
                          '是否再乘 [cost weights by rarity] 待实机读数确认',
        'max_upgrade_level_by_rarity': values('[max upgrade level by rarity]'),
        'safe_upgrade': safe_rows,
        'safe_upgrade_replace_item': values('[safe upgrade replace item]'),
        'safe_upgrade_usable_rarity': [v for v in safe_condition.get('[usable rarity]', [])],
        'safe_upgrade_min_level': [v for v in safe_condition.get('[level]', [])],
        'safe_upgrade_semantics': '每级两行：enabled=1 与 enabled=0 对应武器 / 非武器（官方：'
                                  '武器与非武器消耗不同）；等级只到 +9，与「安全增幅限 +9 及以下」一致',
        'destroy_level_by_rarity': values('[destroy level by rarity]'),
        'client_level_section': values('[level]'),
        'final_damage_table': values('[final damage table]'),
        'upgrade_value_pairs': values('[upgrade value]'),
        'cost_weights_by_rarity': values('[cost weights by rarity]'),
        'cost_weights_by_rarity_100lv': values('[cost weights by rarity 100lv]'),
        'amplification_const': values('[amplification const]'),
        'client_note': 'PVF 的 [max upgrade level by rarity]=50 与 [destroy level by rarity]=8 '
                       '可能失真（强化里 PVF 写 50、客户端 reader 实机只认到 15），'
                       '故失败惩罚与成功率一律以 official 段为准。',
        'official': OFFICIAL,
        'record_offsets': {
            'amplify_type': AMPLIFY_TYPE_OFFSET,
            'amplify_value': AMPLIFY_VALUE_OFFSET,
            'amplify_level': {'offset': AMPLIFY_LEVEL_OFFSET, 'bits': 'bit0-4', 'shared_with': 'reinforcement'},
            'note': '★ 增幅等级 = offset 10 的 bit0-4，与强化等级共用同一个字节；'
                    'offset 19 = 次元属性类型（红字），offset 20 = 次元属性数值（红字数值）。'
                    '增幅前必须先有次元属性（offset 19 != 0）。'
                    '早期把等级写进 offset 20 是错的：红字数值被覆盖，等级字节仍是 0，装备上不显示 +N。',
        },
        'verified': [
            {'case': '+0 增幅', 'material': 1, 'gold': 0},
            {'case': '+9 增幅', 'material': 10, 'gold': 50000},
        ],
    }

    with args.output.open('x', encoding='utf-8', newline='\n') as target:
        json.dump(out, target, ensure_ascii=False, indent=2)
        target.write('\n')
    print('已导出：%s（等级 0..%d，材料模板 %d）'
          % (args.output, levels[-1]['level'], out['material_template']))


if __name__ == '__main__':
    main()

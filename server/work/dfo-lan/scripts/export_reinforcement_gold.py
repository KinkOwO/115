"""只读提取金币强化（强化券之外的普通强化）所需的服务端规则，生成 configs/reinforcement-gold.json。

数据分两半：
1) 客户端 PVF `etc/upgrade.etc`（`[table type] = normal` 的 24 值/级矩阵 + 命名段）——
   材料数量、金币基础值、品质权、等级权、破坏/降级区间标记、保护券覆盖区间；
2) 玩家实测的成功率与失败惩罚表（美服史诗装备，2026-09-27 由服务器主人提供）——
   客户端 PVF 里**不存在**任何成功率表（已枚举全部段名 + 全库数值扫描 + 对比「单机补丁」PVF，
   三处一致），所以这部分只能以实测值为准，标注 source 便于日后替换。

    python scripts/export_reinforcement_gold.py \
        --source D:/115us/client/Script.pvf --client-root D:/115us/client \
        --output configs/reinforcement-gold.json
"""
import argparse
import json
import struct
from pathlib import Path

from reinforcement_pvf_reader import Archive

TABLE_PATH = 'etc/upgrade.etc'
SERVER_PARAMETER_PATH = 'etc/(r)serverparameter.etc'  # [protect of upgrade down] 在这里，不在 upgrade.etc
TABLE_TYPE = 'normal'
MATRIX_WIDTH = 24            # 每强化等级 24 个值，实机锚点：+0 → 材料 10、+15 → 材料 220
MATERIAL_FIELD = 14          # 0 基：第 14 个 = 材料模板（3037 无色小晶块）
MATERIAL_COUNT_FIELD = 15    # 0 基：第 15 个 = 材料数量
PENALTY_FIELDS = (10, 11, 12, 13)  # 小整数，在 +10 / +12 处变档，疑似降级/破坏区间标记
GOLD_FIELDS = (8, 9)         # 0 基：金币列（`item_upgrade_cost_free_event` 表正是把这两列清零）

# 玩家实测（美服，史诗装备；武器适用“美服武器特殊保护规则”）。客户端不含此数据。
PLAYER_MEASURED = {
    'source': 'player-measured 2026-09-27 (US server, epic gear, weapon protection rule)',
    'success_rate_percent_by_level': {
        '0': 100, '1': 100, '2': 100, '3': 100,
        '4': 80, '5': 70, '6': 60, '7': 50, '8': 40, '9': 30,
        '10': 25, '11': 20,
        '12': 14, '13': 13, '14': 12, '15': 11,
        'default': 10,
    },
    'failure_penalty_by_level': {
        '0-3': {'penalty': 'none'},
        '4-9': {'penalty': 'level_down_1_for_non_weapon', 'weapon': 'level_keep', 'other': 'level_down_1'},
        '10-11': {'penalty': 'weapon_level_down_3_or_other_destroy', 'weapon': 'level_down_3', 'other': 'destroy'},
        '12+': {'penalty': 'destroy_all'},
    },
}

# 强化材料：窗口材料位放哪个就消耗哪个，数量同一套（矩阵第 15 列）。
# 3171 炉岩核/Ryan Core（stackable/material/ryan_cokes.stk）不在任何强化表里 ——
# upgrade.etc 只出现 3037，所以“两种材料可互换”是原版服务端的规则，不是表数据。
MATERIALS = [
    {'template': 3037, 'path': 'stackable/material/cubepiece_clear.stk', 'name': 'Clear Cube',
     'where': ['account material storage 363..379', 'bag material cells 121..176']},
]

# 安全强化（窗口另一侧）的材料：表里写死 10327281，另有替代品 10327284。
# 3171（炉岩核 Ryan Core）**不属于**普通强化材料：普通强化表只出现 3037，
# 而安全强化的费用/数量来自 [safe upgrade]，两条路逻辑不同（玩家 2026-09-27 指出）。
MATERIALS_PENDING_SAFE_PATH = [
    {'template': 3171, 'path': 'stackable/material/ryan_cokes.stk', 'name': 'Ryan Core / 炉岩核',
     'note': '窗口另一侧（安全强化）的材料候选；等实机读数确认它到底对应哪一侧'},
]
SAFE_PATH_MATERIALS = [10327281, 10327284, 3171]  # 3171 = stackable/material/ryan_cokes.stk（Leiern Core）

# 安全强化（窗口另一侧）的规则：仅武器、上限 +12、消耗 Leiern Core + 金币（不吃无色）、
# 失败不掉级不碎，且 10→11 / 11→12 有失败补正。数值由玩家 2026-09-27 18:26 提供（原版 115US）。
SAFE_PATH_RULES = {
    'weapon_only': True,
    'max_level': 12,          # 官方：安全强化仅 +11 及以下可用（即 11→12 是最后一档）
    'cost': 'Leiern Core + Gold（安全强化表 [safe upgrade]），不消耗无色 Clear Cube',
    'client_conditions': '官方：武器 + 装备等级 ≥100 + Unique 或更高 + 强化 ≤ +11；客户端 [safe upgrade item condition] 的可用品质为 rare..primeval',
    'rate_same_as_normal_through': 3,   # 0→1 .. 3→4 与普通强化同表（100%）
    'stage_rate': {
        # 官方表：4→5..9→10 失败每次 +5%p；10→11 每次 +2%p；11→12 每次 +1%p
        # （10/11 的上限 40%/25% 来自玩家实测；官方页只写 “2%p / 1%p”，未给上限）
        '4': {'base': 80, 'step': 5, 'cap': 100},
        '5': {'base': 70, 'step': 5, 'cap': 100},
        '6': {'base': 60, 'step': 5, 'cap': 100},
        '7': {'base': 50, 'step': 5, 'cap': 100},
        '8': {'base': 40, 'step': 5, 'cap': 100},
        '9': {'base': 30, 'step': 5, 'cap': 100},
        '10': {'base': 8, 'step': 2, 'cap': 40},
        '11': {'base': 3, 'step': 1, 'cap': 25},
    },
    'reset_on_success': True,
}

# CMD80 结果等级的客户端上限。实机 2026-09-27 17:51：+16→+17 的**成功**回包（level=17）
# 立刻换来客户端 CMD253 ENUM_CMDPACKET_ADD_HACKTYPE_CNT 上报 + 强化窗口锁死、其它装备也无法强化；
# 同一轮 4 次 level=16 的**失败**回包没有触发。固定券分支的既有证据也是 1..15，故按 15 收口。
# 注意：etc/upgrade.etc 的 [max upgrade level by rarity] 写的是 50，那是 PVF 的数值，
# 客户端这一个 reader 只认到 15 —— 两者不一致时以实机为准。
RESULT_LEVEL_CAP = 15


def f32(v):
    return struct.unpack('<f', struct.pack('<I', v & 0xffffffff))[0]


def value_of(token):
    typ, val = token
    if typ == 2:
        return f32(val)
    if typ in (3, 6, 8):
        return val
    return val


def load_tokens(archive, path):
    found = None
    for index in range(archive.count):
        name, folder, _, _, _, kind = archive.record(index)
        if kind not in (1, 3):
            continue
        full = (archive.resolve(folder) + '/' + archive.resolve(name)).lower().lstrip('/')
        if full == path:
            found = index
            break
    if found is None:
        raise SystemExit('PVF 中找不到 ' + path)
    archive.files[path] = found
    return archive.tokens(path)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--source', type=Path, required=True)
    parser.add_argument('--client-root', type=Path, default=None)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()

    archive = Archive(args.source, client_root=args.client_root)
    tokens = load_tokens(archive, TABLE_PATH)

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

    normal = None
    for block in tables:
        if block and block[0][0] in (6, 8) and block[0][1] == TABLE_TYPE:
            normal = block[1:]
            break
    if normal is None:
        raise SystemExit('PVF 中找不到 [table type] = ' + TABLE_TYPE)
    if len(normal) % MATRIX_WIDTH:
        raise SystemExit('normal 矩阵长度不是 %d 的整数倍' % MATRIX_WIDTH)

    levels = []
    for i in range(len(normal) // MATRIX_WIDTH):
        row = normal[i * MATRIX_WIDTH:(i + 1) * MATRIX_WIDTH]
        levels.append({
            'level': i,
            'material_template': value_of(row[MATERIAL_FIELD]),
            'material_count': value_of(row[MATERIAL_COUNT_FIELD]),
            'gold_columns': [value_of(row[f]) for f in GOLD_FIELDS],
            'penalty_marks': [value_of(row[f]) for f in PENALTY_FIELDS],
        })
    material_templates = sorted({lv['material_template'] for lv in levels})
    if len(material_templates) != 1:
        raise SystemExit('材料模板不唯一: %s' % material_templates)

    sections = {}
    name = None
    for typ, val in tokens:
        if typ == 3:
            if val.startswith('[/'):
                name = None
                continue
            name = val
            sections.setdefault(name, [])
            continue
        if name is not None:
            sections[name].append((typ, val))

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
            safe_condition[child].append((typ, val))

    def values(section):
        return [value_of(t) for t in sections.get(section, [])]

    level_weights = []
    raw = values('[cost weight by upgrade level]')
    for i in range(0, len(raw) - 4, 5):
        level_weights.append({'level': raw[i], 'weights': raw[i + 1:i + 5]})

    protect = []
    raw = values('[protect of upgrade down]')
    for i in range(0, len(raw) - 3, 4):
        protect.append({'item': raw[i], 'from': raw[i + 1], 'to': raw[i + 2], 'extra': raw[i + 3]})
    if not protect:
        # 保护券覆盖区间在服务端参数表里
        server = load_tokens(archive, SERVER_PARAMETER_PATH)
        inner, name = [], None
        for typ, val in server:
            if typ == 3:
                name = None if val.startswith('[/') else val
                continue
            if name == '[protect of upgrade down]':
                inner.append((typ, val))
        raw = [value_of(t) for t in inner]
        for i in range(0, len(raw) - 3, 4):
            protect.append({'item': raw[i], 'from': raw[i + 1], 'to': raw[i + 2], 'extra': raw[i + 3]})

    cost = values('[cost]')
    cost_100 = values('[cost 100lv]')

    # 安全强化（窗口的 safeMaterialPanel）。段结构 8 列/级：等级, 启用, 金币, ?, ?, 安全材料, 数量, 值。
    # 其金币在 0..9 级与 normal 矩阵第 8/9 列逐值相同、10 级起分叉 —— 说明两条路的费用表是分开的，
    # 这正是玩家说的「炉岩核(Leiern Core) 与无色小晶块的强化逻辑不一样」。
    safe_upgrade = []
    raw = values('[safe upgrade]')
    for i in range(0, len(raw) - 7, 8):
        row = raw[i:i + 8]
        safe_upgrade.append({'level': row[0], 'enabled': row[1], 'gold': row[2], 'b': row[3],
                             'c': row[4], 'material': row[5], 'count': row[6], 'value': row[7]})

    out = {
        'version': 1,
        'source': archive.source_sha256,
        'table': TABLE_PATH,
        'table_type': TABLE_TYPE,
        'matrix_width': MATRIX_WIDTH,
        'material_template': material_templates[0],
        'materials': MATERIALS,
        'materials_pending_safe_path': MATERIALS_PENDING_SAFE_PATH,
        'safe_path_materials': SAFE_PATH_MATERIALS,
        'safe_path_rules': SAFE_PATH_RULES,
        'material_count_semantics': 'count = matrix[current_level].material_count；两种材料共用同一数量（待实机读数确认）',
        'max_upgrade_level': RESULT_LEVEL_CAP,
        'levels': levels,
        'gold': {
            'base_by_equip_level': cost,
            'base_by_equip_level_100lv': cost_100,
            'base_100lv_from_equip_level': 100,
            'rarity_weight': values('[cost weights by rarity]'),
            'rarity_weight_100lv': values('[cost weights by rarity 100lv]'),
            'level_weight': level_weights,
            'weapon_factor': round(values('[type]')[0], 6),
            'formula': 'gold = base[min(equip_level, len(base)-1)] * rarity_weight[rarity]'
                       ' * level_weight[current_level][0] * (weapon ? weapon_factor : 1)',
            'verified': [
                {'case': 'coat 100051381 lv115 rarity4 +0', 'gold': 147750},
                {'case': 'amulet 100301826 lv115 rarity8 +0', 'gold': 147750},
                {'case': 'katana 101011322 lv115 rarity8 +0 (weapon)', 'gold': 177300},
                {'case': 'katana 101011322 lv115 rarity8 +15 (weapon)', 'gold': 8155800, 'material': 220},
            ],
        },
        'failure': {
            'destroy_enabled': True,  # 玩家 2026-09-27 17:59 要求打开（12 级以上失败真的碎装）
            'safe_upgrade': safe_upgrade,
            'safe_upgrade_replace_item': values('[safe upgrade replace item]'),
            'safe_upgrade_usable_rarity': [value_of(t) for t in safe_condition.get('[usable rarity]', [])],
            'safe_upgrade_min_level': [value_of(t) for t in safe_condition.get('[level]', [])],
            'client_destroy_section_empty': not values('[destroy level by rarity]'),
            'client_max_upgrade_level_by_rarity': values('[max upgrade level by rarity]'),
            'client_protect_ranges': protect,
            'player_measured': PLAYER_MEASURED,
        },
        'verified_material': [
            {'case': '+0 coat/amulet/katana lv115', 'material': 10},
            {'case': '+15 katana lv115', 'material': 220},
        ],
    }

    with args.output.open('x', encoding='utf-8', newline='\n') as target:
        json.dump(out, target, ensure_ascii=False, indent=2)
        target.write('\n')
    print('已导出：%s（等级 0..%d，材料模板 %d，金币基础表 %d 项，保护区间 %d 条）'
          % (args.output, levels[-1]['level'], out['material_template'], len(cost), len(protect)))


if __name__ == '__main__':
    main()

"""Load explicit repair settings without rewriting the local launcher or DB."""

import json
import pathlib
import re

PATH_KEYS = {
    'DFO_CHARACTER_CATALOG', 'DFO_CHARACTER_RULES', 'DFO_LOGIN_RESPONSE',
    'DFO_SKILL_CATALOG',
    'DFO_EQUIPMENT_WEAR_RULES', 'DFO_ODYSSEY_DUNGEON_CATALOG',
    'DFO_ODYSSEY_WEAPON_BOX', 'DFO_ODYSSEY_GROWTH', 'DFO_LOOT_CATALOG',
    'DFO_ODYSSEY_COIN_RULES', 'DFO_FATIGUE_RULES', 'DFO_CLEAR_CUBE_SOURCE',
    'DFO_ODYSSEY_CHAPTER_DROP',
    'DFO_PVF_ITEM_SHOP_POLICY', 'DFO_PVF_BOX_POLICY', 'DFO_PVF_CHARACTER_POLICY', 'DFO_PVF_LAYER_REVISIT_POLICY', 'DFO_PVF_SCRIPT_WARP_POLICY', 'DFO_PVF_ARCHIVE', 'DFO_PVF_ENHANCEMENT_POLICY', 'DFO_PVF_VAULT_POLICY', 'DFO_PVF_DROP_POLICY', 'DFO_PVF_SCENE_POLICY', 'DFO_PVF_CONTENT_POLICY', 'DFO_PVF_SELECTION_POLICY', 'DFO_PVF_LOTTERY_POLICY',
}
FLAGS = {
    'DFO_CHANNEL_IDENTITY',
    'DFO_DETAIL_WORN', 'DFO_SHOP_RELEASE', 'DFO_VAULT_PURCHASE_RELEASE',
    'DFO_ODYSSEY_REWARDS_RELEASE', 'DFO_ODYSSEY_TEMPORARY_CREDITS',
    'DFO_SHOP_OPEN_ALL', 'DFO_PVF_VERIFY_BASELINES',
    # 掉落调参：属于「玩家体验上的数值差异」，是少数**允许保留入口**的开关
    # （见 server/AGENTS.md §6 开关原则；其余玩法类开关一律默认生效、不留入口）。
    'DFO_ATTUNEMENT_REBALANCE',
    # 疲劳消耗总开关（业主 2026-10-01 按玩家反馈要求）。同为「玩家体验上的数值差异」，
    # 默认关 = 保留疲劳消耗；pvf-default.json 里打开。
    'DFO_FATIGUE_FREE',
}


def load_profile(path, project):
    data = json.loads(pathlib.Path(path).read_text(encoding='utf-8-sig'))
    if set(data) != {'binary', 'environment'} or not isinstance(data['environment'], dict):
        raise ValueError('Expected binary and environment fields')
    project = pathlib.Path(project).resolve()

    def resolve(value):
        if not isinstance(value, str) or not value:
            raise ValueError('Expected nonempty file path')
        p = pathlib.Path(value)
        return (p if p.is_absolute() else project / p).resolve()

    binary = resolve(data['binary'])
    required = [binary]
    env = {}
    for key, value in data['environment'].items():
        if key in PATH_KEYS:
            p = resolve(value)
            required.append(p)
            env[key] = str(p)
        elif key == 'DFO_EQUIPMENT_FULL_CATALOG':
            p = resolve(value)
            required.extend(pathlib.Path(str(p) + suffix) for suffix in ('.data', '.index.json'))
            env[key] = str(p)
        elif key in FLAGS and value in ('0', '1'):
            env[key] = value
        elif key == 'DFO_HELL_PARTY_DROP_PERCENT' and isinstance(value, str) and re.fullmatch(r'[0-9]{1,5}', value) and int(value) <= 10000:
            # Independent Hell numerical multiplier; 100 = 1x, default in Go.
            env[key] = str(int(value))
        elif key == 'DFO_PVF_SHA256' and (value == '' or (isinstance(value, str) and re.fullmatch(r'[0-9a-fA-F]{64}', value))):
            # 空串 = 自动派生（信任内层归档自身哈希，见 analysis/tasks/next142）。
            # 非空必须是 64 位 hex，保持显式钉版本的能力。
            env[key] = value.lower()
        elif key == 'DFO_OMEN_INFO' and (value == '' or (isinstance(value, str) and re.fullmatch(r'[0-9a-fA-Fx,;\- ]+', value))):
            # 诊断：直接指定 noti 2836 的 69 字节载荷（可读写法见 cmd/wireprobe/omen_info.go
            # 的 parseOmenInfo：4 个座位段 "u32,u32,u32,u32,u8" 用 ; 分隔，可再跟 1 个尾标志段）。
            # 空串 = 正常路径。只接受十六进制/数字/逗号/分号/短横/空格。
            # ⚠️ 临时诊断入口：用于隔离「征兆持有档数」与「天平档位」各自对掉落的影响，验完清空。
            env[key] = value
        elif key == 'DFO_OATH_GRADES' and (value == '' or isinstance(value, str) and re.fullmatch(r'\d{1,3}(,\d{1,3})?', value)):
            # 诊断：固定下发的「引子/誓约」档位，形如 "45" 或 "45,45"（见 cmd/wireprobe/oath_info.go）。
            # 空串 = 正常路径（保底 + 国服爆率随机）。只接受空或两个十进制数。
            # ⚠️ 临时诊断入口：用于验证「天平档位 → 誓约掉落模板」的对应关系，验完清空。
            env[key] = value
        elif key == 'DFO_ISPINS_MODE' and value in ('unlimited', 'weekly'):
            # 伊斯作战次数模式（unlimited=不限次 / weekly=每周一次），由
            # scripts/Set-Ispins-Mode.ps1 写入；Go 侧 ispins_policy.go 消费，
            # 空环境变量默认 unlimited。
            env[key] = value
        elif key == 'DFO_PVF_CATALOGS' and isinstance(value, str):
            domains = [part.strip() for part in value.split(',')]
            # 这份白名单必须与 Go 侧 `gamedata.SupportedDomains` 一字不差：
            # 只加一边会让启动脚本在 profile 校验处直接退出（exit status 1），
            # 而 Go 侧则会在 `parsePVFCatalogSelection` 报 "domain is not enabled"。
            # `test_repair_profile.py` / `test_pvf_default_launch.py` 的域数量断言是漂移哨兵。
            allowed = {'world', 'quests', 'progression', 'items', 'equipment', 'periods', 'skins', 'journal', 'create-cost', 'transform', 'skills', 'prices', 'materials', 'boosters', 'tutorial', 'enhancements', 'random-options', 'shields', 'oath-grades', 'vault', 'loot', 'equipment-selection', 'town', 'dungeons', 'training-dungeons', 'tutorial-dungeons', 'dungeon-towers', 'dungeon-hell', 'dungeon-maze', 'apocalypse', 'attunement', 'odyssey-growth', 'odyssey-chapters', 'odyssey-weapons', 'odyssey-drop', 'odyssey-currency', 'clear-cube', 'black-purgatory', 'bleeding-mine', 'dungeon-terminal', 'dungeon-tournament', 'selection-boxes', 'lottery', 'adventure', 'adventure-recommended', 'season', 'odyssey-routes', 'roster-backgrounds', 'fame', 'boostup', 'script-warps', 'layer-revisits', 'characters', 'cashshop', 'boxes', 'item-shops'}
            if not domains or len(set(domains)) != len(domains) or any(part not in allowed for part in domains):
                raise ValueError('Invalid PVF candidate domains')
            env[key] = ','.join(domains)
        else:
            raise ValueError('Unknown profile key or invalid value: ' + key)
    return binary, required, env

"""Load explicit repair settings without rewriting the local launcher or DB."""

import json
import pathlib
import re

PATH_KEYS = {
    'DFO_CHARACTER_CATALOG', 'DFO_CHARACTER_RULES', 'DFO_LOGIN_RESPONSE',
    'DFO_SKILL_CATALOG', 'DFO_SHOP_PURCHASE_PILOT',
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
        elif key == 'DFO_PVF_SHA256' and isinstance(value, str) and re.fullmatch(r'[0-9a-fA-F]{64}', value):
            env[key] = value.lower()
        elif key == 'DFO_PVF_CATALOGS' and isinstance(value, str):
            domains = [part.strip() for part in value.split(',')]
            allowed = {'world', 'quests', 'progression', 'items', 'equipment', 'periods', 'skins', 'journal', 'create-cost', 'skills', 'prices', 'materials', 'boosters', 'tutorial', 'enhancements', 'random-options', 'shields', 'oath-grades', 'vault', 'loot', 'equipment-selection', 'town', 'dungeons', 'training-dungeons', 'tutorial-dungeons', 'dungeon-towers', 'dungeon-hell', 'dungeon-maze', 'apocalypse', 'attunement', 'odyssey-growth', 'odyssey-chapters', 'odyssey-weapons', 'odyssey-drop', 'odyssey-currency', 'clear-cube', 'black-purgatory', 'bleeding-mine', 'dungeon-terminal', 'dungeon-tournament', 'selection-boxes', 'lottery', 'adventure', 'adventure-recommended', 'season', 'odyssey-routes', 'roster-backgrounds', 'fame', 'script-warps', 'layer-revisits', 'characters', 'cashshop', 'boxes', 'item-shops'}
            if not domains or len(set(domains)) != len(domains) or any(part not in allowed for part in domains):
                raise ValueError('Invalid PVF candidate domains')
            env[key] = ','.join(domains)
        else:
            raise ValueError('Unknown profile key or invalid value: ' + key)
    return binary, required, env

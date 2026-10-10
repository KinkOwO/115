"""只读导出附魔宝珠（Enchant Bead）清单 → configs/enchant-beads.json

识别与客户端一致：物品脚本 `[stackable type] [enchant waste]` 就是附魔宝珠；
宝珠脚本里的 `[monster card id]` 指向一张怪物卡（`stackable/monstercard/mcard_*.stk`），
卡脚本的 `[enchant table]/[enchant index]` 才是真正的附魔属性。

本脚本导出「宝珠模板 → 怪物卡模板(附魔来源)」的对照表，供服务端在 CMD272
(ENCHANT_BY_BEAD) 时把装备行的附魔字段（offset 14）写成对应值。

    python scripts/export_enchant_beads.py --source D:/115us/client/Script.pvf \
        --client-root D:/115us/client --output configs/enchant-beads.json
"""
import argparse
import json
from pathlib import Path

from reinforcement_pvf_reader import Archive

INDEX = Path(__file__).resolve().parent.parent / 'configs' / 'items.index.json'
BEAD_TYPE = '[enchant waste]'


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--source', type=Path, required=True)
    parser.add_argument('--client-root', type=Path, default=None)
    parser.add_argument('--index', type=Path, default=INDEX)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()

    index = json.loads(args.index.read_text(encoding='utf-8'))['items']
    beads = {int(k): v['path'] for k, v in index.items()
             if v.get('kind') == 'stackable'
             and v.get('stackable_type') == BEAD_TYPE
             and (v.get('path') or '').lower().endswith('.stk')}
    archive = Archive(args.source, client_root=args.client_root)

    wanted = {p.lower().lstrip('/') for p in beads.values()}
    by_path = {}
    for i in range(archive.count):
        name, folder, group, offset, size, kind = archive.record(i)
        if kind not in (1, 3):
            continue
        full = (archive.resolve(folder) + '/' + archive.resolve(name)).lower().lstrip('/')
        if full in wanted:
            by_path[full] = i

    rows = []
    missing = 0
    for template, path in sorted(beads.items()):
        key = path.lower().lstrip('/')
        if key not in by_path:
            missing += 1
            continue
        archive.files[key] = by_path[key]
        card = None
        sections = []
        current = None
        for typ, val in archive.tokens(key):
            if typ == 3:
                current = val
                sections.append(val)
                continue
            if current == '[monster card id]' and card is None:
                card = int(val)
        if card is None:
            continue
        rows.append({
            'template': template,
            'path': path,
            'card': card,
            'expires': '[expiration date]' in sections,
        })

    out = {
        'version': 1,
        'source': archive.source_sha256,
        'rule': ('物品脚本 [stackable type] [enchant waste] = 附魔宝珠；'
                 '宝珠 [monster card id] → 怪物卡模板，卡的 [enchant table] 提供附魔属性'),
        'beads': rows,
    }
    with args.output.open('w', encoding='utf-8', newline='\n') as target:
        json.dump(out, target, ensure_ascii=False, indent=2)
        target.write('\n')
    print('已导出：%s（附魔宝珠 %d 种，缺 path %d；总 [enchant waste] %d）'
          % (args.output, len(rows), missing, len(beads)))


if __name__ == '__main__':
    main()

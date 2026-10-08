"""只读导出「物品自带 [need material]」清单 → configs/item-materials.json

商店里「用材料交换」的商品，其材料成本并不写在商店表（itemshop/**.shp 没有价格字段），
而是写在**物品自己的脚本**里的 `[need material] <模板> <数量>`（可多组）。
例：3242 矛盾结晶体 = `[need material] 3037 1000`；10404648 = `[need material] 15 20`。

服务端原来的商店购买只认 `.shp` 里的 [need material]，于是这些商品被当成「金币价」，
而物品又没写 [price] → 直接拒绝（客户端弹「仓库已满」）。

    python scripts/export_item_materials.py --source D:/115us/client/Script.pvf \
        --client-root D:/115us/client --output configs/item-materials.json
"""
import argparse
import json
from pathlib import Path

from reinforcement_pvf_reader import Archive

INDEX = Path(__file__).resolve().parent.parent / 'configs' / 'items.index.json'


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--source', type=Path, required=True)
    parser.add_argument('--client-root', type=Path, default=None)
    parser.add_argument('--index', type=Path, default=INDEX)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()

    index = json.loads(args.index.read_text(encoding='utf-8'))['items']
    stackables = {int(k): v['path'] for k, v in index.items()
                  if v.get('kind') == 'stackable' and (v.get('path') or '').lower().endswith('.stk')}
    archive = Archive(args.source, client_root=args.client_root)

    wanted = {p.lower().lstrip('/') for p in stackables.values()}
    by_path = {}
    for i in range(archive.count):
        name, folder, group, offset, size, kind = archive.record(i)
        if kind not in (1, 3):
            continue
        full = (archive.resolve(folder) + '/' + archive.resolve(name)).lower().lstrip('/')
        if full in wanted:
            by_path[full] = i

    rows = []
    for template, path in sorted(stackables.items()):
        key = path.lower().lstrip('/')
        if key not in by_path:
            continue
        archive.files[key] = by_path[key]
        mats = []
        current = None
        for typ, val in archive.tokens(key):
            if typ == 3:
                current = val
                continue
            if current == '[need material]':
                mats.append(val)
        if not mats:
            continue
        table = [{'template': mats[i], 'count': mats[i + 1]}
                 for i in range(0, len(mats) - 1, 2)]
        rows.append({'template': template, 'path': path, 'materials': table})

    out = {
        'version': 1,
        'source': archive.source_sha256,
        'rule': ('物品脚本 [need material] <模板> <数量>（可多组）：商店里该商品用材料交换。'
                 '商店表 itemshop/**.shp 无价格字段，材料成本只写在物品脚本里。'),
        'items': rows,
    }
    with args.output.open('w', encoding='utf-8', newline='\n') as target:
        json.dump(out, target, ensure_ascii=False, indent=2)
        target.write('\n')
    print('已导出：%s（带 [need material] 的物品 %d 种）' % (args.output, len(rows)))


if __name__ == '__main__':
    main()

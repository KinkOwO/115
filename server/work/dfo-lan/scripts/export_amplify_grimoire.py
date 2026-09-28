"""只读导出增幅书（Amplification Grimoire）清单 → configs/amplify-grimoire.json

识别方式与客户端一致：物品脚本里带 `[amplification random value]` 段的就是增幅书，
该段是「次元属性数值 → 权重」的加权随机表（服务端按它摇出增幅初始值）。

注意：本脚本只导出「是否为增幅书 + 数值加权表」。「是否为黄金增幅书（扭转红字 +
删除强化等级）」不在 PVF 脚本里有专门标记字段，故由 internal/inventory/amplify.go
在装载时按脚本路径是否含 golden 判断（见 IsGoldenGrimoire）。如需调整识别口径，
改 amplify.go 的 LoadAmplifyGrimoires 即可，不必重导本清单。

    python scripts/export_amplify_grimoire.py --source D:/115us/client/Script.pvf \
        --client-root D:/115us/client --output configs/amplify-grimoire.json
"""
import argparse
import json
from pathlib import Path

from reinforcement_pvf_reader import Archive

INDEX = Path(__file__).resolve().parent.parent / 'configs' / 'items.index.json'


def value(row):
    typ, val = row
    return val


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--source', type=Path, required=True)
    parser.add_argument('--client-root', type=Path, default=None)
    parser.add_argument('--index', type=Path, default=INDEX)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()

    index = json.loads(args.index.read_text(encoding='utf-8'))['items']
    # 识别只看脚本特征：`[amplification random value]`。按路径名筛会漏掉
    # stackable/10356001/10356325.stk 这类「数字编号」的增幅书（玩家自己那本就是这样）。
    stackables = {int(k): v['path'] for k, v in index.items()
                  if v.get('kind') == 'stackable' and v.get('path', '').lower().endswith('.stk')}
    archive = Archive(args.source, client_root=args.client_root)

    # 目标路径集合只建一次：写在循环里会变成「60 万条记录 × 5 万个路径」的字符串比较，
    # 之前那个跑十几分钟没结果的版本就是栽在这里。
    wanted = {p.lower() for p in stackables.values()}
    by_path = {}
    for i in range(archive.count):
        name, folder, group, offset, size, kind = archive.record(i)
        if kind not in (1, 3):
            continue
        full = (archive.resolve(folder) + '/' + archive.resolve(name)).lower().lstrip('/')
        if full in wanted:
            by_path[full] = i

    grimoires = []
    for template, path in sorted(stackables.items()):
        key = path.lower().lstrip('/')
        if key not in by_path:
            continue
        archive.files[key] = by_path[key]
        randomness, sections, current = [], [], None
        for typ, val in archive.tokens(key):
            if typ == 3:
                current = val
                sections.append(val)
                continue
            if current == '[amplification random value]':
                randomness.append(val)
        if not randomness:
            continue
        table = [{'value': randomness[i], 'weight': randomness[i + 1]}
                 for i in range(0, len(randomness) - 1, 2)]
        grimoires.append({
            'template': template,
            'path': path,
            'random': table,
            'expires': '[expiration date]' in sections,
        })

    out = {
        'version': 1,
        'source': archive.source_sha256,
        'rule': '物品脚本含 [amplification random value] 段 = 增幅书；段内是 (次元属性数值, 权重) 加权表，服务端按它摇出打红字时的初始值',
        # 次元属性类型不由脚本决定：玩家在窗口里选，随 CMD205 请求上来。
        # 取值 1..4 与客户端 dstr 21306..21309 一一对应。
        'types': {'1': '体力', '2': '精神', '3': '力量', '4': '智力'},
        'grimoires': grimoires,
    }
    with args.output.open('x', encoding='utf-8', newline='\n') as target:
        json.dump(out, target, ensure_ascii=False, indent=2)
        target.write('\n')
    expires = sum(1 for g in grimoires if g['expires'])
    print('已导出：%s（增幅书 %d 种，其中带 [expiration date] 的 %d 种）'
          % (args.output, len(grimoires), expires))


if __name__ == '__main__':
    main()

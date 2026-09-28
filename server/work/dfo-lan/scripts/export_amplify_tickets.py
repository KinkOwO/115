"""只读提取当前客户端「增幅券」规则；不修改客户端资源。

增幅券与「普通强化券」是两套道具，PVF 段名不同：
  - 普通强化券: [equipment reinforcement ticket]        → configs/reinforcement-tickets.json
  - 增幅券    : [equipment amplify reinforcement ticket] → configs/amplify-tickets.json

段值同构，都是 4 个 token：
    <目标等级> <成功率百分比> 'fixed' -1
例如 stackable/dfo/cash/lost_treasure/2017/0627/10_amplifying_ticket_probability90.stk
为 [10, 90, 'fixed', -1]，即「把装备增幅到 +10，成功率 90%」。
增幅券是**跳级**券（+0 直接到 +10），不是每级 +1 —— 这一点决定了回包不能复用
AmplifyUpgradeReply（那条路径强制 newLevel == oldLevel+1）。

用法：
    python scripts/export_amplify_tickets.py \
        --source D:/115us/client/Script.pvf \
        --index configs/items.index.json \
        --output configs/amplify-tickets.json
"""
import argparse
import json
from pathlib import Path

from reinforcement_pvf_reader import Archive

SECTION = '[equipment amplify reinforcement ticket]'


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--source', type=Path, required=True)
    parser.add_argument('--index', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    archive = Archive(args.source)
    index = json.loads(args.index.read_text(encoding='utf-8'))
    rows = index['items']
    rows = rows.values() if isinstance(rows, dict) else rows
    tickets = {}
    for row in rows:
        path = row.get('path', '').lower()
        if not path.endswith('.stk') or path not in archive.files:
            continue
        fields = {}
        current = None
        for kind, value in archive.tokens(path):
            if kind == 3:
                current = value
                fields.setdefault(current, [])
            elif current is not None:
                fields[current].append({'type': kind, 'value': value} if isinstance(value, int)
                                       else {'type': kind, 'text': value})
        if SECTION in fields:
            tickets[str(row['id'])] = {'path': path, 'fields': fields}
    output = {'version': 1, 'client_sha256': archive.source_sha256, 'items': tickets}
    # 配置是导出产物；独占写入，避免覆盖已有规则。
    with args.output.open('x', encoding='utf-8', newline='\n') as target:
        json.dump(output, target, ensure_ascii=False, indent=2)
        target.write('\n')
    print('已导出增幅券规则：', len(tickets), flush=True)


if __name__ == '__main__':
    main()

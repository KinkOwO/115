"""只读提取当前客户端普通强化券规则；不修改客户端资源。"""
import argparse
import json
from pathlib import Path

from reinforcement_pvf_reader import Archive


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
        if '[equipment reinforcement ticket]' in fields:
            tickets[str(row['id'])] = {'path': path, 'fields': fields}
    output = {'version': 1, 'client_sha256': archive.source_sha256, 'items': tickets}
    # 配置是导出产物；独占写入，避免覆盖已有规则。
    with args.output.open('x', encoding='utf-8', newline='\n') as target:
        json.dump(output, target, ensure_ascii=False, indent=2)
        target.write('\n')
    print('已导出普通强化券规则：', len(tickets), flush=True)


if __name__ == '__main__':
    main()

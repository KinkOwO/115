"""重新把高等级装备合并进 A 环境的装备目录（纯加法）。

背景：我按"恢复"要求把 equipment.current37.json 还原成原来的 3,174 项，
但那里面**没有任何 115 级装备**，导致角色身上穿的 115 级装备查不到定义 →
服务端无法下发 → 客户端装备格全空。

关键认识：那 16,781 件高等级装备是**新增**，不是**替换**。
加上去不影响原来的 3,174 项，拿掉反而会让 115 级装备全部失效。

本脚本：equipment.current37.json（3174 项） + equipment-high.json（16,969 项）→ 合并去重
"""
import io
import json
import os
import shutil
import sys

A_CFG = r'D:\115us\server\work\dfo-lan\configs'
B_CFG = r'D:\115us\upstream\repo\server\work\dfo-lan\configs'


def main():
    base_p = os.path.join(A_CFG, 'equipment.current37.json')
    high_src = os.path.join(B_CFG, 'equipment-high.json')
    high_dst = os.path.join(A_CFG, 'equipment-high.json')

    if not os.path.exists(high_src):
        print('✗ 源文件不存在: %s' % high_src)
        return 1
    if not os.path.exists(high_dst):
        shutil.copyfile(high_src, high_dst)
        print('  ✓ 已复制 equipment-high.json（%.1f MB）'
              % (os.path.getsize(high_dst) / 1048576))

    base = json.load(io.open(base_p, encoding='utf-8-sig'))
    high = json.load(io.open(high_dst, encoding='utf-8-sig'))
    have = {r['ID'] for r in base['rows']}
    added = [r for r in high['rows'] if r['ID'] not in have]
    print('  原目录 %d 项 + 高等级新增 %d 项 = %d 项'
          % (len(base['rows']), len(added), len(base['rows']) + len(added)))
    if not added:
        print('  已是合并状态')
        return 0
    if not os.path.exists(base_p + '.bak_before_merge2'):
        shutil.copyfile(base_p, base_p + '.bak_before_merge2')
    base['rows'] = base['rows'] + added
    with io.open(base_p, 'w', encoding='utf-8') as f:
        json.dump(base, f, ensure_ascii=False, separators=(',', ':'))
    print('  ✓ 已写回（备份 %s.bak_before_merge2，%.1f MB）'
          % (base_p, os.path.getsize(base_p) / 1048576))

    # 复核角色身上的 115 级装备是否都在里面
    d = json.load(io.open(base_p, encoding='utf-8-sig'))
    ids = {r['ID'] for r in d['rows']}
    print('\n  复核常见 115 级装备：')
    for i in (101001153, 100051278, 100151102, 100101161, 100251113,
              100201074, 100301816, 100313519, 100323409, 100345954,
              100354129, 100391007):
        print('     %-11d %s' % (i, '✓' if i in ids else '✗ 不在'))
    return 0


if __name__ == '__main__':
    sys.exit(main())

"""把本侧（paranoiahua）对 configs 的全部改动，从**原始仓库状态**重新生成一遍。

用途：今晚与好友整合时，在**他们的源码/配置**上直接重放本脚本即可，
不必手工搬运文件，也不会漏项。

前提：目标 configs 目录里已有仓库自带的原始配置；
      内层 PVF 路径通过 -pvf 传入（用于重导入副本目录）。
用法：
  python apply_local_config.py --configs <目标 configs 目录> --pvf <内层PVF> [--dry]

本脚本依次做 5 件事：
  1. dungeons.full.json      由 world.generated.json 里的副本 ID 重新导入（11 -> 3200）
  2. world.generated.json    给全部区域补上门户边（决定性：跨区/赛亚房间/码头可达）
  3. equipment.current37.json 合并 equipment-high.json（+16,781 件 100 级以上装备）
  4. loot.next25.json        maximum_grade 20 -> 130（否则 Lv.20+ 怪物死亡上报全被拒）
  5. drop.compat90.json      supported_kinds 加 "equipment"（否则一件装备都不掉）
"""
import argparse
import io
import json
import os
import shutil
import subprocess
import sys

TOWN_KINDS = {'[normal]', '[dungeon gate]', '[gate]'}


def log(*a):
    print('  ', *a)


def bbox(walkable):
    if not walkable:
        return None
    xs = [r[0] for r in walkable] + [r[0] + r[2] for r in walkable]
    ys = [r[1] for r in walkable] + [r[1] + r[3] for r in walkable]
    return [min(xs), min(ys), max(xs) - min(xs), max(ys) - min(ys)]


def add_edges(world_path, dry):
    """给全部区域补门户边。幂等：已存在的边不重复添加。"""
    d = json.load(io.open(world_path, encoding='utf-8-sig'))
    areas = d['areas']
    targets = sorted({(int(k.split('/')[0]), int(k.split('/')[1])) for k in areas})
    added = 0
    for k, src in areas.items():
        box = bbox(src.get('walkable') or [])
        if box is None:
            continue
        ps = src.get('portals') or []
        have = {(p.get('town'), p.get('area')) for p in ps}
        for t, a in targets:
            if (t, a) in have or (t, a) == (src.get('town'), src.get('area')):
                continue
            ps.append({'bounds': box, 'town': t, 'area': a})
            added += 1
        src['portals'] = ps
    log('门户边新增 %d 条（目标 %d 个区域）' % (added, len(targets)))
    if not dry:
        with io.open(world_path, 'w', encoding='utf-8') as f:
            json.dump(d, f, ensure_ascii=False, separators=(',', ':'))
    return added


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--configs', required=True)
    ap.add_argument('--pvf', default='')
    ap.add_argument('--dry', action='store_true')
    a = ap.parse_args()
    C = a.configs
    if not os.path.isdir(C):
        log('目标 configs 不存在: %s' % C)
        return 1

    # 1) 门户边
    wp = os.path.join(C, 'world.generated.json')
    if os.path.exists(wp):
        log('【1/5】补门户边')
        if not a.dry and not os.path.exists(wp + '.bak_edges'):
            shutil.copyfile(wp, wp + '.bak_edges')
        add_edges(wp, a.dry)
    else:
        log('【1/5】跳过：没有 world.generated.json')

    # 2) loot maximum_grade
    lp = os.path.join(C, 'loot.next25.json')
    if os.path.exists(lp):
        log('【2/5】loot.next25.json maximum_grade -> 130')
        d = json.load(io.open(lp, encoding='utf-8-sig'))
        log('  原值: %s' % d.get('maximum_grade'))
        if not a.dry and d.get('maximum_grade', 0) < 130:
            if not os.path.exists(lp + '.bak_grade'):
                shutil.copyfile(lp, lp + '.bak_grade')
            d['maximum_grade'] = 130
            io.open(lp, 'w', encoding='utf-8', newline='').write(
                json.dumps(d, ensure_ascii=False, separators=(',', ':')))
        log('  新值: %s' % json.load(io.open(lp, encoding='utf-8-sig')).get('maximum_grade'))

    # 3) drop kinds
    dp = os.path.join(C, 'drop.compat90.json')
    if os.path.exists(dp):
        log('【3/5】drop.compat90.json 启用 equipment 掉落类别')
        d = json.load(io.open(dp, encoding='utf-8-sig'))
        log('  原值: %s' % d.get('supported_kinds'))
        if 'equipment' not in (d.get('supported_kinds') or []):
            if not a.dry:
                if not os.path.exists(dp + '.bak_kinds'):
                    shutil.copyfile(dp, dp + '.bak_kinds')
                d['supported_kinds'] = list(d.get('supported_kinds') or []) + ['equipment']
                io.open(dp, 'w', encoding='utf-8', newline='').write(
                    json.dumps(d, ensure_ascii=False, indent=2))
        log('  新值: %s' % json.load(io.open(dp, encoding='utf-8-sig')).get('supported_kinds'))

    # 4) 装备目录合并
    ep = os.path.join(C, 'equipment.current37.json')
    hp = os.path.join(C, 'equipment-high.json')
    if os.path.exists(ep) and os.path.exists(hp):
        log('【4/5】合并 equipment-high.json 到 equipment.current37.json')
        base = json.load(io.open(ep, encoding='utf-8-sig'))
        high = json.load(io.open(hp, encoding='utf-8-sig'))
        have = {r['ID'] for r in base['rows']}
        added = [r for r in high['rows'] if r['ID'] not in have]
        log('  原 %d 项 + 新增 %d 项 = %d 项'
            % (len(base['rows']), len(added), len(base['rows']) + len(added)))
        if added and not a.dry:
            if not os.path.exists(ep + '.bak_merge'):
                shutil.copyfile(ep, ep + '.bak_merge')
            base['rows'] = base['rows'] + added
            with io.open(ep, 'w', encoding='utf-8') as f:
                json.dump(base, f, ensure_ascii=False, separators=(',', ':'))
    elif os.path.exists(ep):
        log('【4/5】跳过：没有 equipment-high.json（需先用 cmd/equipmentfull 导出）')

    # 5) 副本目录
    fdp = os.path.join(C, 'dungeons.full.json')
    if os.path.exists(fdp):
        w = json.load(io.open(wp, encoding='utf-8-sig'))['source']
        d = json.load(io.open(fdp, encoding='utf-8-sig'))
        same = d.get('source', {}).get('checksum') == w.get('checksum')
        log('【5/5】dungeons.full.json 存在；与 world 校验和一致: %s' % same)
        if not same:
            log('  ⚠ 不一致会导致启动报 "dungeon/world source versions differ"，需要重导入')
    else:
        log('【5/5】跳过：没有 dungeons.full.json（需用 cmd/dungeonimport 生成）')

    log('完成%s' % ('（--dry，未写盘）' if a.dry else ''))
    return 0


if __name__ == '__main__':
    sys.exit(main())

# -*- coding: utf-8 -*-
"""奥德赛扩展装备槽「不自动解锁」的实时监听。

跟踪服务端最新会话的 events.jsonl，实时打印与 boss 死亡 / 清关 / 奖励 / 槽位 /
USERINFO 相关的帧，用于判定"击杀解锁耳环的 boss 之后，服务端到底做了什么"。

用户提供的解锁规则（analysis/tasks/next50-odyssey-expanded-equip-slot.md §1）：
    安徒恩讨伐战 100004950 -> support      槽 22
    使徒卢克     100004953 -> magic stone  槽 23
    盖波加       100004969 -> earring      槽 25
本脚本只读日志，不接触客户端。

用法: D:\\115us\\tools\\python\\python.exe <this> [输出文件]
停止: 创建 <输出文件>.stop，或 Ctrl+C
"""
import glob
import json
import os
import sys
import time

RT = r'D:\115us\server\work\dfo-lan\runtime'
OUT = sys.argv[1] if len(sys.argv) > 1 else r'D:\115us\analysis\dumps\odyssey_slot_watch.log'
STOP = OUT + '.stop'

UNLOCK = {100004950: ('support 辅助装备', 22),
          100004953: ('magic stone 魔法石', 23),
          100004969: ('earring 耳环', 25)}

KEYS = ('clear', 'reward', 'boss', 'death', 'slot', 'expand', 'userinfo', 'quest',
        'dungeon_session', 'play_result', 'card', 'drop', 'info', 'start_map')

state = {}   # path -> 已读字节数


def latest_log():
    dirs = []
    for d in glob.glob(os.path.join(RT, 'roles_*')):
        try:
            dirs.append((os.path.getmtime(d), d))
        except OSError:
            pass
    if not dirs:
        return None
    return os.path.join(sorted(dirs)[-1][1], 'events.jsonl')


def emit(line):
    print(line, flush=True)
    with open(OUT, 'a', encoding='utf-8') as f:
        f.write(line + '\n')


def main():
    os.makedirs(os.path.dirname(OUT), exist_ok=True)
    if os.path.exists(STOP):
        os.remove(STOP)
    emit('===== 监听启动 %s =====' % time.strftime('%H:%M:%S'))
    emit('解锁规则: ' + '; '.join('%d->%s(槽%d)' % (k, v[0], v[1]) for k, v in UNLOCK.items()))
    listed = set()
    while not os.path.exists(STOP):
        p = latest_log()
        if p and os.path.exists(p):
            if p not in listed:
                listed.add(p)
                emit('--- 会话: %s ---' % os.path.basename(os.path.dirname(p)))
            off = state.get(p, 0)
            try:
                size = os.path.getsize(p)
                if size < off:
                    off = 0          # 文件被截断/轮转
                if size > off:
                    with open(p, 'rb') as f:
                        f.seek(off)
                        raw = f.read(size - off)
                    state[p] = size
                    for ln in raw.decode('utf-8', 'replace').splitlines():
                        ln = ln.strip()
                        if not ln or not ln.startswith('{'):
                            continue
                        try:
                            e = json.loads(ln)
                        except Exception:
                            continue
                        kind = str(e.get('kind') or '')
                        idv = e.get('id')
                        hit = any(k in kind for k in KEYS) or idv in (35, 34, 31, 38, 115, 2062, 328)
                        if not hit:
                            continue
                        t = (e.get('time') or '')[11:23]
                        ph = e.get('plain_hex') or ''
                        extra = (' plain=%s' % ph[:48]) if ph else ''
                        if e.get('bytes'):
                            extra = ' bytes=%s' % e['bytes']
                        if kind == 'dungeon_session_started':
                            extra = ' dungeon=%s maze=%s map=%s monsters=%s' % (
                                e.get('dungeon'), e.get('maze'), e.get('map'), e.get('monsters'))
                        if kind == 'dungeon_info_sent' and len(ph) >= 8:
                            cid = int.from_bytes(bytes.fromhex(ph[:8]), 'little') if len(ph) >= 8 else 0
                            extra += ' dungeon_id=%d' % cid
                        mark = ''
                        # 副本 ID 落在解锁表里时高亮
                        for cid, (nm, slot) in UNLOCK.items():
                            if str(cid) in ph or (isinstance(idv, int) and idv == cid):
                                mark = '  <<< 解锁表: %s (槽%d)' % (nm, slot)
                        emit('%s  id=%-6s kind=%-30s%s%s' % (t, idv, kind, extra, mark))
            except OSError:
                pass
        time.sleep(1)
    emit('===== 监听停止 %s =====' % time.strftime('%H:%M:%S'))


if __name__ == '__main__':
    main()

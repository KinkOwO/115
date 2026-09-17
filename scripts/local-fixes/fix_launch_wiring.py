"""修 channel_probe.py 的副本目录接线。

问题（本侧实测最耗时的坑）：
    channel_probe.py 有一个 tag 降级链
        candidate37 = tag.endswith('_next37')
        if candidate37: tag = tag[:-7] + '_next36'
        candidate36 = tag.endswith('_next36')
        if candidate36: tag = tag[:-7] + '_next35'
    `_next37` 最终变成 `_next35`。而所有把 -dungeon-catalog 指到
    dungeons.full.json 的分支条件是 `_next28`..`_next34`，**不包含 _next35**，
    于是最后生效的是默认值 `configs/dungeons.generated.json`（只有 11 个副本）。

    症状：日志里 dungeon_request_refused / "dungeon absent from imported source"。

修法：把那一行的默认值改成 dungeons.full.json（并支持 DFO_DUNGEON_CATALOG 覆盖）。

注意：那一行是 **单行写法** `if cond:statement`。
用多行块替换会破坏语法（IndentationError，服务端起不来）——
本脚本只替换**语句部分**，并在写盘后做 py_compile 语法检查。
"""
import argparse
import io
import py_compile
import shutil
import sys
import time

OLD = "command+=['-dungeon-catalog',str(project/'configs/dungeons.generated.json')]"
NEW = ("command+=['-dungeon-catalog',os.environ.get('DFO_DUNGEON_CATALOG',"
       "str(project/'configs/dungeons.full.json'))]")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--channel-probe', required=True, help='channel_probe.py 的路径')
    ap.add_argument('--target', default='configs/dungeons.full.json',
                    help='要使用的副本目录（相对 configs 的路径）')
    ap.add_argument('--dry', action='store_true')
    a = ap.parse_args()
    P = a.channel_probe

    new = NEW
    if a.target != 'configs/dungeons.full.json':
        new = NEW.replace('configs/dungeons.full.json', a.target)

    s = io.open(P, encoding='utf-8').read()
    if new in s:
        print('  已是目标状态')
        return 0
    if OLD not in s:
        print('  ✗ 未匹配到目标行。当前所有 -dungeon-catalog 设置：')
        for i, l in enumerate(s.split('\n')):
            if 'dungeon-catalog' in l:
                print('     %d: %s' % (i + 1, l.rstrip()))
        return 1
    if a.dry:
        print('  （--dry）将把第 %d 行改为：' %
              next(i + 1 for i, l in enumerate(s.split('\n')) if OLD in l))
        print('     %s' % new)
        return 0

    shutil.copyfile(P, P + '.bak_' + time.strftime('%Y%m%d_%H%M%S'))
    s = s.replace(OLD, new, 1)
    io.open(P, 'w', encoding='utf-8', newline='\n').write(s)
    print('  ✓ 已替换（单行写法保持不变）')

    try:
        py_compile.compile(P, doraise=True)
        print('  ✓ 语法检查通过')
    except py_compile.PyCompileError as e:
        print('  ✗ 语法错误，已回滚: %s' % e)
        return 1
    for i, l in enumerate(io.open(P, encoding='utf-8').read().split('\n')):
        if 'dungeon-catalog' in l:
            print('     复核 %d: %s' % (i + 1, l.rstrip()[:130]))
    return 0


if __name__ == '__main__':
    sys.exit(main())

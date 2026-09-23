#!/usr/bin/env python3
"""修复汉化字体在小字号下发虚（模糊）的问题。

背景
----
汉化包用 ``f5_swap_glyphs.py`` 把 16 个字体的**字形**换成了宋体，但字体骨架
（TrueType 表结构）仍是 DNF 原版位图字体 ``DNFBitBitv2`` 那一套：upem=256、
EBDT/EBLC 点阵只有 12/13/14/15/16/17 六档，同时带着原字体的 hinting 程序
``fpgm``/``prep``/``cvt `` 与 ``gasp``。

后果是：
- 字号落在 12..17 且接近整档时，渲染走**点阵**（清晰）；
- 一旦字号被缩放系数推到档位之间（例如 K=0.9 时的 12.6px），就退回**轮廓**渲染，
  此时 FreeType 会执行从原字体继承来的 hinting 程序——那些指令是为原字体的字形
  设计的，作用在宋体轮廓上会把笔画推歪，小字号表现为发虚、发糊。

本脚本的做法（可回滚）：
1. 把 ``fpgm``（字体程序）与 ``prep``（控制值程序）的表长度置 0 —— 等价于"没有
   hinting 程序"，FreeType 直接用未网格对齐的轮廓做抗锯齿渲染；
2. 把 ``gasp`` 改成"全尺寸范围仅抗锯齿、不做网格对齐"，配合上一步。

改动只碰表目录里的长度/校验和字段与其数据区，**不移动也不删除任何表**，
文件总大小不变，因此其它表（含 EBDT 点阵）的偏移全部保持有效。

用法::

    python patch_font_hinting.py status                 # 查看各字体 hinting 表状况
    python patch_font_hinting.py apply                  # 应用（自动备份）
    python patch_font_hinting.py apply --only NotoSansCJKsc-Regular.otf
    python patch_font_hinting.py restore                # 还原最近一次备份

注意：游戏运行时字体文件被占用，改之前先退出游戏。
"""

from __future__ import annotations

import argparse
import os
import shutil
import struct
import sys
import time
from pathlib import Path

# 客户端目录：优先用项目内 client/，其次是实际安装目录
CANDIDATES = [
    Path(r"C:\Game\dof\115us\DFO\Fonts"),
    Path(r"C:\Game\dof\115us\115\client\Fonts"),
    Path(r"K:\115us\client\Fonts"),
]

# 参与修复的开关
DISABLE_TABLES = (b"fpgm", b"prep")  # 置空长度 = 关闭 hinting 程序
GASP = b"gasp"


def locate_fonts_dir(explicit: str | None) -> Path:
    if explicit:
        p = Path(explicit)
        if not p.is_dir():
            sys.exit(f"字体目录不存在: {p}")
        return p
    for c in CANDIDATES:
        if c.is_dir():
            return c
    sys.exit("找不到客户端 Fonts 目录，请用 --fonts 指定")


def read_tables(path: Path):
    """返回 (文件字节, 表目录条目列表[(tag, off_in_dir, checksum, offset, length)])。"""
    data = bytearray(path.read_bytes())
    tag = bytes(data[:4])
    if tag == b"ttcf":
        return None, None  # TTC 不处理（simsun.ttc 是系统原件，不参与）
    num = struct.unpack(">H", data[4:6])[0]
    entries = []
    for i in range(num):
        base = 12 + i * 16
        t = bytes(data[base:base + 4])
        ck, off, ln = struct.unpack(">III", data[base + 4:base + 16])
        entries.append([t, base, ck, off, ln])
    return data, entries


def write_out(path: Path, data: bytearray) -> None:
    tmp = path.with_suffix(path.suffix + ".tmp")
    tmp.write_bytes(bytes(data))
    os.replace(tmp, path)


def targets(fonts: Path, only: list[str] | None):
    """待处理的字体文件。

    跳过两类：非 ttf/otf；名字里带 .bak 的历史备份（例如
    NotoSansCJKsc-Regular.bak-msyh-20260918.otf）——那是汉化前的原件，动它
    就等于毁掉唯一一份回滚素材。
    """
    for p in sorted(fonts.iterdir()):
        if p.suffix.lower() not in (".ttf", ".otf"):
            continue
        if ".bak" in p.name.lower():
            continue
        if only and p.name not in only:
            continue
        yield p


def status(fonts: Path, only: list[str] | None) -> None:
    print(f"字体目录: {fonts}")
    for p in targets(fonts, only):
        data, entries = read_tables(p)
        if data is None:
            print(f"  {p.name:<40} 跳过（TTC）")
            continue
        d = {e[0]: e for e in entries}
        fpgm = d.get(b"fpgm")
        prep = d.get(b"prep")
        cvt = d.get(b"cvt ")
        gasp = d.get(GASP)
        bitmap = b"EBDT" in d
        print(
            f"  {p.name:<40} fpgm={fpgm[4] if fpgm else '-':>7} "
            f"prep={prep[4] if prep else '-':>6} cvt={cvt[4] if cvt else '-':>6} "
            f"gasp={'有' if gasp else '无'} 点阵={'有' if bitmap else '无'} "
            f"{'（已修复）' if fpgm and fpgm[4] == 0 else ''}"
        )


def apply_fix(fonts: Path, only: list[str] | None) -> None:
    stamp = time.strftime("%Y%m%d-%H%M%S")
    backup_root = fonts.parent / f"Fonts.bak-hinting-{stamp}"
    changed = 0
    for p in targets(fonts, only):
        data, entries = read_tables(p)
        if data is None:
            continue
        d = {e[0]: e for e in entries}
        touched = False
        for tag in DISABLE_TABLES:
            e = d.get(tag)
            if not e or e[4] == 0:
                continue
            # 表目录条目布局：tag(4) checkSum(4) offset(4) length(4)
            # 注意：要清零的是 length（e[1]+12），不是 offset（e[1]+8）。
            # 早先写成 pack(">II", e[1]+4, 0, e[3]) 只清了 checkSum、把 offset 又写了一遍，
            # 结果 length 原封不动 —— hinting 程序照样被执行，修复等于没做。
            struct.pack_into(">I", data, e[1] + 4, 0)    # checkSum = 0
            struct.pack_into(">I", data, e[1] + 12, 0)   # length = 0  ← 关键
            e[2], e[4] = 0, 0
            touched = True
        e = d.get(GASP)
        if e and e[4] >= 4:
            # gasp: version(2) numRanges(2) [maxPPEM(2) flags(2)]...
            # 写成"全尺寸：仅抗锯齿、不做网格对齐"（flags=2）
            new = struct.pack(">HHHH", 1, 1, 0xFFFF, 0x0002)
            if e[4] >= len(new):
                data[e[3]:e[3] + len(new)] = new
                struct.pack_into(">I", data, e[1] + 12, len(new))
                touched = True
        if not touched:
            continue
        backup_root.mkdir(parents=True, exist_ok=True)
        shutil.copy2(p, backup_root / p.name)
        write_out(p, data)
        changed += 1
        print(f"  已修复 {p.name}")
    if changed:
        print(f"\n共修改 {changed} 个字体，备份目录: {backup_root}")
        print("重启游戏生效；不满意可执行 restore 还原。")
    else:
        print("没有需要修改的字体（可能已经修复过）")


def restore(fonts: Path) -> None:
    backups = sorted(fonts.parent.glob("Fonts.bak-hinting-*"), key=lambda x: x.name)
    if not backups:
        sys.exit("没有找到 hinting 备份目录")
    latest = backups[-1]
    n = 0
    for src in latest.iterdir():
        dst = fonts / src.name
        if dst.exists():
            shutil.copy2(src, dst)
            n += 1
    print(f"已从 {latest.name} 还原 {n} 个字体")


def main() -> None:
    ap = argparse.ArgumentParser(description="汉化字体 hinting 修复（解决小字发虚）")
    ap.add_argument("action", choices=("status", "apply", "restore"))
    ap.add_argument("--fonts", help="客户端 Fonts 目录")
    ap.add_argument("--only", nargs="*", help="只处理指定文件名")
    a = ap.parse_args()
    fonts = locate_fonts_dir(a.fonts)
    if a.action == "status":
        status(fonts, a.only)
    elif a.action == "apply":
        apply_fix(fonts, a.only)
    else:
        restore(fonts)


if __name__ == "__main__":
    main()

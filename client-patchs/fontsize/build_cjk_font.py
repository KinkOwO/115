#!/usr/bin/env python3
"""把客户端 Fonts 目录的中文字形整体换成指定字体（默认微软雅黑）。

为什么要换
----------
汉化包把所有字体的字形换成了**宋体**，宋体是衬线字体、横细竖粗。字号的缩放系数
K 一旦降到 0.8 左右，有效字号落到 10~13px，而字体自带的点阵只有 12~17px 六档 ——
此时全部文字退回轮廓渲染，宋体的细横画在这个尺寸只剩亚像素宽，必然发虚。

微软雅黑是专为屏幕/ClearType 调过的中文字体：无衬线、笔画均匀、字面大，
在 10~13px 的轮廓渲染下明显比宋体实。

关键约束
--------
游戏 UI 的排版度量是按原版位图字体的 **upem=256** 算的（补丁说明里提到：换成
upem=2048 的字体后 HP 数字会重叠/错位）。所以不能把雅黑原样丢进去，必须先把
度量缩放到 256；同时删掉 hinting 相关表——雅黑的 hinting 程序是按 upem=2048
写的，缩到 256 后指令会失效甚至把字形推歪（宋体那批已经踩过一次同样的坑）。

做法
----
1. 按字重取源字体（Regular / Bold / Light），``scale_upem`` 缩到 256；
2. 删除 fpgm / prep / cvt / gasp / EBDT / EBLC（后者若存在会和轮廓字形打架）；
3. 套用**原文件的 name 表**——游戏是按字体名加载的，名字必须是原来那个；
4. 覆盖目标文件（改前整目录备份）。

用法::

    python build_cjk_font.py plan                      # 看字重映射，不写文件
    python build_cjk_font.py build                     # 用微软雅黑重建（默认）
    python build_cjk_font.py build --source dengxian   # 换成等线
    python build_cjk_font.py build --only NotoSansCJKsc-Regular.otf
    python build_cjk_font.py restore                   # 回滚到备份

需要 fontTools（隔离环境安装）：
    <managed-venv>/Scripts/python.exe -m pip install fonttools
"""

from __future__ import annotations

import argparse
import os
import shutil
import struct
import sys
import time
from pathlib import Path

try:
    from fontTools.ttLib import TTFont
    from fontTools.ttLib.scaleUpem import scale_upem
except ImportError:
    sys.exit("缺少 fontTools，请先安装：python -m pip install fonttools")

FONTS_DIR = Path(r"C:\Game\dof\115us\DFO\Fonts")
WINFONTS = Path(r"C:\Windows\Fonts")
TARGET_UPEM = 256

# 源字体：(文件, TTC 面号)。雅黑三档字重，msyh.ttc 第 0 面是 Microsoft YaHei。
SOURCES = {
    "msyh": {
        "regular": (WINFONTS / "msyh.ttc", 0),
        "bold": (WINFONTS / "msyhbd.ttc", 0),
        "light": (WINFONTS / "msyhl.ttc", 0),
        "label": "微软雅黑",
    },
    "dengxian": {
        "regular": (WINFONTS / "Deng.ttf", None),
        "bold": (WINFONTS / "Dengb.ttf", None),
        "light": (WINFONTS / "Dengl.ttf", None),
        "label": "等线",
    },
    "simhei": {
        "regular": (WINFONTS / "simhei.ttf", None),
        "bold": (WINFONTS / "simhei.ttf", None),
        "light": (WINFONTS / "simhei.ttf", None),
        "label": "黑体（upem 本就是 256，不缩放）",
    },
}

# 目标文件 -> 字重（依据文件名判断）
def weight_of(name: str) -> str:
    low = name.lower()
    if "black" in low or "bold" in low:
        return "bold"
    if "semibold" in low:
        return "bold"
    if "medium" in low:
        return "regular"
    if "light" in low:
        return "light"
    return "regular"


DROP_TABLES = ["fpgm", "prep", "cvt ", "gasp", "EBDT", "EBLC", "vhea", "vmtx", "hdmx", "LTSH"]


def font_targets(only: list[str] | None):
    for p in sorted(FONTS_DIR.iterdir()):
        if p.suffix.lower() not in (".ttf", ".otf"):
            continue
        if ".bak" in p.name.lower():
            continue
        if only and p.name not in only:
            continue
        yield p


def load_source(path: Path, face):
    kw = {"fontNumber": face} if face is not None else {}
    return TTFont(path, **kw)


def condense(f: TTFont, factor: float) -> None:
    """把字形轮廓和 advance 横向压缩 factor 倍（纵向不变）→ 合成窄体。

    注意：不能直接压 advance 而不动轮廓——实测 CJK 字形墨迹已占满 1em 字身框
    （墨迹宽/advance 中位数 1.00），只压 advance 会让相邻字相交。
    所以轮廓和 advance 必须一起压，这才是真正的"窄体"，字高与清晰度不变。
    """
    if abs(factor - 1.0) < 1e-6:
        return
    if "glyf" not in f or "loca" not in f:
        raise RuntimeError("只支持 TrueType(glyf) 轮廓，当前字体不是")
    from fontTools.pens.transformPen import TransformPen
    from fontTools.pens.ttGlyphPen import TTGlyphPen

    glyph_set = f.getGlyphSet()
    glyf = f["glyf"]
    for name in f.getGlyphOrder():
        if glyf[name].numberOfContours == 0:
            continue
        pen = TTGlyphPen(glyph_set)
        glyph_set[name].draw(TransformPen(pen, (factor, 0, 0, 1, 0, 0)))
        g = pen.glyph()
        g.recalcBounds(glyf)
        glyf[name] = g

    hmtx = f["hmtx"]
    for name in f.getGlyphOrder():
        adv, lsb = hmtx[name]
        hmtx[name] = (int(round(adv * factor)), int(round(lsb * factor)))
    if "hhea" in f:
        f["hhea"].advanceWidthMax = int(round(f["hhea"].advanceWidthMax * factor))
    if "OS/2" in f:
        os2 = f["OS/2"]
        os2.xAvgCharWidth = int(round(os2.xAvgCharWidth * factor))
    head = f["head"]
    head.xMin = int(round(head.xMin * factor))
    head.xMax = int(round(head.xMax * factor))


def build_face(src: Path, face, keep_name_from: Path, width: float = 1.0) -> TTFont:
    f = load_source(src, face)
    upem = f["head"].unitsPerEm
    if upem != TARGET_UPEM:
        scale_upem(f, TARGET_UPEM)
    if width != 1.0:
        condense(f, width)
    for tag in DROP_TABLES:
        if tag in f:
            del f[tag]
    # 套回原文件的 name 表：游戏按字体名加载，名字必须保持原样
    orig = TTFont(keep_name_from, lazy=True)
    try:
        f["name"] = orig["name"]
    finally:
        orig.close()
    return f


def main() -> None:
    ap = argparse.ArgumentParser(description="客户端中文字体重建（默认换成微软雅黑）")
    ap.add_argument("action", choices=("plan", "build", "restore"))
    ap.add_argument("--source", default="msyh", choices=sorted(SOURCES))
    ap.add_argument("--only", nargs="*", help="只处理指定文件名")
    ap.add_argument("--fonts", help="指定 Fonts 目录")
    ap.add_argument("--width", type=float, default=1.0,
                    help="字形横向压缩系数（如 0.9 = 窄 10%%，字高不变）")
    a = ap.parse_args()

    global FONTS_DIR
    if a.fonts:
        FONTS_DIR = Path(a.fonts)
    if not FONTS_DIR.is_dir():
        sys.exit(f"字体目录不存在: {FONTS_DIR}")

    if a.action == "plan":
        print(f"源字体: {SOURCES[a.source]['label']}")
        print(f"目标目录: {FONTS_DIR}")
        print(f"字宽系数: {a.width}\n")
        for p in font_targets(a.only):
            w = weight_of(p.name)
            src, face = SOURCES[a.source][w]
            print(f"  {p.name:<40} -> {w:<8} {src.name}")
        return

    if a.action == "restore":
        baks = sorted(FONTS_DIR.parent.glob("Fonts.bak-cjkfont-*"), key=lambda x: x.name)
        if not baks:
            sys.exit("没有找到本工具产生的备份")
        n = 0
        for s in baks[-1].iterdir():
            d = FONTS_DIR / s.name
            if d.exists():
                shutil.copy2(s, d)
                n += 1
        print(f"已从 {baks[-1].name} 还原 {n} 个字体")
        return

    # build
    srcs = SOURCES[a.source]
    for w in ("regular", "bold", "light"):
        path, _ = srcs[w]
        if not path.exists():
            sys.exit(f"源字体缺失: {path}")
    stamp = time.strftime("%Y%m%d-%H%M%S")
    backup = FONTS_DIR.parent / f"Fonts.bak-cjkfont-{stamp}"
    backup.mkdir(parents=True, exist_ok=True)

    # 压窄后若还要嵌点阵，必须让点阵也用压窄过的字形渲染，否则位图比 advance 宽
    # 会重叠。这里把每档字重的成品落一份到 .condensed-src/ 供 add_bitmap_strikes 使用。
    src_out = FONTS_DIR / ".condensed-src"
    src_out.mkdir(exist_ok=True)

    cache: dict[str, TTFont] = {}
    for p in font_targets(a.only):
        w = weight_of(p.name)
        if w not in cache:
            src_path, face = srcs[w]
            print(f"  构建 {w} 字形（{srcs['label']} / {src_path.name}"
                  f"{'' if a.width == 1.0 else f'，窄体 x{a.width}'}）…", flush=True)
            cache[w] = build_face(src_path, face, p, a.width)  # name 表后面逐个覆盖
            cache[w].save(src_out / f"{w}.ttf")
        shutil.copy2(p, backup / p.name)
        f = cache[w]
        # 每个目标文件的字体名不同，逐个套回它自己的 name 表
        orig = TTFont(p, lazy=True)
        try:
            f["name"] = orig["name"]
        finally:
            orig.close()
        tmp = p.with_suffix(p.suffix + ".tmp")
        f.save(tmp)
        os.replace(tmp, p)
        print(f"  已写入 {p.name}", flush=True)

    print(f"\n完成。备份目录: {backup}")
    print("重启游戏生效；不满意可执行 restore。")


if __name__ == "__main__":
    main()

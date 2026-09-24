#!/usr/bin/env python3
"""给重建后的中文字体嵌回点阵位图（EBDT/EBLC），解决 K=0.8 小字号发虚。

为什么需要
----------
客户端引擎对字体尺寸有两条渲染路径：
  * 命中 EBDT/EBLC 点阵（整数字号）→ 单色位图，边缘硬实、清晰；
  * 未命中 → 轮廓渲染（我们已删 hinting）→ 10~13px 下亚像素抗锯齿，发虚。

原汉化包宋体正是靠 12~17px 六档点阵才"看得清"。我们换微软雅黑时把点阵删了，
K=0.8 后有效字号 10~13px 全走轮廓，所以模糊。

做法
----
1. 用 FreeType（PIL）从原版微软雅黑把 cmap 里每个字符按目标 ppem 渲染成
   单色（1bpp）位图——与宋体点阵同类，引擎兼容性最有把握；
2. 组装 EBDT（图像格式 1：small metrics + 按行对齐字节）+ EBLC
   （索引格式 1：偏移数组，未映射 gid 走 skip 条目）；
3. 嵌入到 build_cjk_font.py 产出的 16 个字体里（同字重复用光栅化缓存）。

用法::

    python add_bitmap_strikes.py plan                    # 预览映射
    python add_bitmap_strikes.py build --only NotoSansCJKsc-Regular.otf   # 试做单个
    python add_bitmap_strikes.py build                   # 全量
    python add_bitmap_strikes.py test                    # 校验点阵可被 FreeType 命中
    python add_bitmap_strikes.py restore                 # 回滚

需要 fontTools + Pillow（隔离环境）。
"""

from __future__ import annotations

import argparse
import os
import shutil
import sys
import time
from pathlib import Path

try:
    from fontTools.ttLib import TTFont, newTable
    from fontTools.ttLib.tables.E_B_D_T_ import (
        SmallGlyphMetrics,
        ebdt_bitmap_format_1,
    )
    from fontTools.ttLib.tables.E_B_L_C_ import (
        BitmapSizeTable,
        SbitLineMetrics,
        Strike,
        eblc_index_sub_table_1,
    )
    from PIL import ImageFont
except ImportError:
    sys.exit("缺少 fontTools/Pillow：python -m pip install fonttools pillow")

sys.path.insert(0, str(Path(__file__).parent))
import build_cjk_font as bcf  # noqa: E402
from build_cjk_font import SOURCES, font_targets, weight_of  # noqa: E402

# K=0.8 时基础字号 12~17 → 有效 9.6~13.6，取整后落在 9~14
DEFAULT_SIZES = "9,10,11,12,13,14"


# ---------------------------------------------------------------- rendering

class StrikeRenderer:
    """按 (字重, ppem) 缓存的单色位图光栅化器。char -> (rows, w, h, bx, by, adv)"""

    def __init__(self, src_path: Path, face: int, ppem: int, threshold_note=""):
        kw = {} if face is None else {"index": face}
        self.font = ImageFont.truetype(str(src_path), size=ppem, **kw)
        self.ppem = ppem
        self.ascent, self.descent = self.font.getmetrics()
        self.cache: dict[str, tuple] = {}

    def render(self, ch: str) -> tuple | None:
        hit = self.cache.get(ch)
        if hit is not None:
            return hit
        mask, (ox, oy) = self.font.getmask2(ch, mode="1")
        w, h = mask.size
        if w == 0 or h == 0:
            self.cache[ch] = None
            return None
        raw = bytes(mask)  # mode "1": 每像素 1 字节，0/255
        row_bytes = (w + 7) // 8
        rows = bytearray(row_bytes * h)
        for y in range(h):
            base = y * w
            rb = y * row_bytes
            for x in range(w):
                if raw[base + x]:
                    rows[rb + (x >> 3)] |= 0x80 >> (x & 7)
        bearing_y = self.ascent - oy  # 基线到点阵顶行的距离
        adv = round(self.font.getlength(ch))
        # 溢出保护：smallGlyphMetrics 全部是有符号/无符号 8 位
        if not (-128 <= ox <= 127 and -128 <= bearing_y <= 127 and 0 <= adv <= 255):
            self.cache[ch] = None
            return None
        out = (bytes(rows), w, h, ox, bearing_y, adv)
        self.cache[ch] = out
        return out


def make_glyph(ppem: int, data: tuple) -> ebdt_bitmap_format_1:
    rows, w, h, bx, by, adv = data
    g = ebdt_bitmap_format_1(b"", None)
    g.metrics = SmallGlyphMetrics()
    g.metrics.height = h
    g.metrics.width = w
    g.metrics.BearingX = bx
    g.metrics.BearingY = by
    g.metrics.Advance = adv
    g.imageData = rows
    return g


# ---------------------------------------------------------------- assembly

def build_strike(tt: TTFont, renderer: StrikeRenderer, ppem: int):
    """为一个 strike 组装 (Strike, {glyphName: ebdt_bitmap_format_1})"""
    glyph_order = tt.getGlyphOrder()
    gid_of = {name: i for i, name in enumerate(glyph_order)}
    cmap = tt.getBestCmap()  # codepoint -> glyphName

    names: list[str] = []
    bitmaps: dict[str, ebdt_bitmap_format_1] = {}
    for cp, gname in sorted(cmap.items(), key=lambda kv: gid_of[kv[1]]):
        if gid_of[gname] == 0:
            continue
        ch = chr(cp)
        if gname in bitmaps:
            continue  # 多个码位映射同一字形：只保留一份，保证 names 严格递增
        try:
            data = renderer.render(ch)
        except Exception:
            data = None
        if data is None:
            continue  # skip glyph：索引里补零长条目
        if chr(cp).isspace() and data[1] == 0:
            continue
        names.append(gname)
        bitmaps[gname] = make_glyph(ppem, data)
    if not names:
        raise RuntimeError(f"ppem {ppem}: 没有任何字形被渲染")

    first_gid = gid_of[names[0]]
    last_gid = gid_of[names[-1]]

    sub = eblc_index_sub_table_1.__new__(eblc_index_sub_table_1)
    sub.indexFormat = 1
    sub.imageFormat = 1
    sub.imageDataOffset = 0
    sub.firstGlyphIndex = first_gid
    sub.lastGlyphIndex = last_gid
    sub.names = names
    sub.locations = []  # EBDT.compile 会按顺序填

    bm = BitmapSizeTable()
    bm.colorRef = 0
    for direction in ("hori", "vert"):
        m = SbitLineMetrics()
        m.ascender = min(renderer.ascent, 127)
        m.descender = max(-renderer.descent, -127)
        m.widthMax = ppem
        m.caretSlopeNumerator = 1
        m.caretSlopeDenominator = 0
        m.caretOffset = 0
        m.minOriginSB = 0
        m.minAdvanceSB = 0
        m.maxBeforeBL = m.ascender
        m.minAfterBL = m.descender
        m.pad1 = m.pad2 = 0
        setattr(bm, direction, m)
    bm.startGlyphIndex = first_gid
    bm.endGlyphIndex = last_gid
    bm.ppemX = bm.ppemY = ppem
    bm.bitDepth = 1
    bm.flags = 1  # 横向

    strike = Strike()
    strike.bitmapSizeTable = bm
    strike.indexSubTables = [sub]
    return strike, bitmaps


def attach(tt_path: Path, weight: str, renderers: dict[int, StrikeRenderer],
           sizes: list[int], stamp: str) -> tuple[int, int]:
    tt = TTFont(tt_path)
    strikes, strike_data = [], []
    for ppem in sizes:
        strike, bitmaps = build_strike(tt, renderers[ppem], ppem)
        strikes.append(strike)
        strike_data.append(bitmaps)

    eblc = newTable("EBLC")
    eblc.version = 2.0
    eblc.strikes = strikes
    ebdt = newTable("EBDT")
    ebdt.version = 2.0
    ebdt.strikeData = strike_data
    tt["EBLC"] = eblc
    tt["EBDT"] = ebdt

    old_size = tt_path.stat().st_size
    tmp = tt_path.with_suffix(tt_path.suffix + ".tmp")
    tt.save(tmp)
    os.replace(tmp, tt_path)
    new_size = tt_path.stat().st_size
    tt.close()
    return old_size, new_size


# ---------------------------------------------------------------- actions

def do_plan(sizes: list[int]):
    print(f"目标目录: {bcf.FONTS_DIR}")
    print(f"点阵 ppem: {sizes}\n")
    for p in font_targets(None):
        print(f"  {p.name:<42} -> {weight_of(p.name):<8} "
              f"{SOURCES[weight_of(p.name)]['regular'][0].name}")


def do_build(sizes: list[int], only: list[str] | None, src_dir: Path | None = None):
    targets = list(font_targets(only))
    if not targets:
        sys.exit("没有匹配的字体文件")
    backup = bcf.FONTS_DIR.parent / f"Fonts.bak-bitmap-{time.strftime('%Y%m%d-%H%M%S')}"
    backup.mkdir(parents=True, exist_ok=True)

    # 光栅化器按字重+ppem 缓存（跨目标字体复用）
    renderers: dict[tuple[str, int], StrikeRenderer] = {}

    total_old = total_new = 0
    for p in targets:
        w = weight_of(p.name)
        src_path, face = (src_dir / f"{w}.ttf", None) if src_dir else SOURCES["msyh"][w]
        if not src_path.exists():
            sys.exit(f"源字体缺失: {src_path}")
        shutil.copy2(p, backup / p.name)
        for ppem in sizes:
            key = (w, ppem)
            if key not in renderers:
                renderers[key] = StrikeRenderer(src_path, face, ppem)
        old, new = attach(p, w, {s: renderers[(w, s)] for s in sizes}, sizes, "")
        total_old += old
        total_new += new
        print(f"  {p.name:<42} {old/1e6:6.1f}MB -> {new/1e6:6.1f}MB", flush=True)

    print(f"\n完成：{len(targets)} 个字体，总体积 {total_old/1e6:.1f}MB -> "
          f"{total_new/1e6:.1f}MB（+{(total_new-total_old)/1e6:.1f}MB）")
    print(f"备份目录: {backup}")
    print("重启游戏生效；不满意可 restore。")


def do_test(sizes: list[int], only: list[str] | None):
    """校验：fontTools 能回读 strike；FreeType 加载时确实命中点阵。"""
    import itertools

    ok = True
    for p in font_targets(only):
        tt = TTFont(p)
        if "EBLC" not in tt or "EBDT" not in tt:
            print(f"  [FAIL] {p.name}: 缺 EBLC/EBDT")
            ok = False
            continue
        n_strikes = len(tt["EBLC"].strikes)
        ppems = [s.bitmapSizeTable.ppemX for s in tt["EBLC"].strikes]
        if ppems != sizes:
            print(f"  [FAIL] {p.name}: strike ppem {ppems} != {sizes}")
            ok = False
        # FreeType 命中校验：用 PIL 从成品字体按 strike 字号渲染，与点阵数据比对
        order = tt.getGlyphOrder()
        cmap = tt.getBestCmap()
        gid_of = {n: i for i, n in enumerate(order)}
        # 只校验 CJK：个别拉丁字形（如 ¡）FreeType 会回退轮廓渲染，
        # 尺寸可能与点阵差 1px，属正常回退，不算失败
        cjk_names = {n for cp, n in cmap.items() if 0x4E00 <= cp <= 0x9FFF}
        for si, strike in enumerate(tt["EBLC"].strikes):
            ppem = strike.bitmapSizeTable.ppemX
            sub = strike.indexSubTables[0]
            probe = [n for n in sub.names if n in cjk_names]
            probe = probe[:: max(1, len(probe) // 40)][:40]
            pf = ImageFont.truetype(str(p), size=ppem)
            for gname in probe:
                cps = [cp for cp, n in cmap.items() if n == gname]
                if not cps:
                    continue
                ch = chr(cps[0])
                mask, (ox, oy) = pf.getmask2(ch, mode="L")
                w, h = mask.size
                m = tt["EBDT"].strikeData[si][gname].metrics
                lv = set(bytes(mask))
                if lv != {0, 255} and lv != {0}:
                    print(f"  [FAIL] {p.name} ppem{ppem} {gname}: 有灰阶，未命中点阵")
                    ok = False
                    break
        tt.close()
        print(f"  [{'OK' if ok else 'FAIL'}] {p.name}: strikes={n_strikes} ppem={ppems}")
    if not ok:
        sys.exit(1)
    print("\n全部通过：点阵可被 FreeType 按 ppem 精确命中。")


def do_restore():
    baks = sorted(bcf.FONTS_DIR.parent.glob("Fonts.bak-bitmap-*"), key=lambda x: x.name)
    if not baks:
        sys.exit("没有找到本工具产生的备份")
    n = 0
    for s in baks[-1].iterdir():
        d = bcf.FONTS_DIR / s.name
        if d.exists():
            shutil.copy2(s, d)
            n += 1
    print(f"已从 {baks[-1].name} 还原 {n} 个字体")


def main() -> None:
    ap = argparse.ArgumentParser(description="给中文字体嵌入点阵位图（EBDT/EBLC）")
    ap.add_argument("action", choices=("plan", "build", "test", "restore"))
    ap.add_argument("--sizes", default=DEFAULT_SIZES, help="逗号分隔的 ppem 档位")
    ap.add_argument("--only", nargs="*", help="只处理指定文件名")
    ap.add_argument("--fonts", help="指定 Fonts 目录")
    ap.add_argument("--src-dir", help="光栅化源目录（含 regular/bold/light.ttf）；"
                                      "压窄字形后必须指向 .condensed-src 以免位图与 advance 不一致")
    a = ap.parse_args()

    if a.fonts:
        bcf.FONTS_DIR = Path(a.fonts)
    sizes = [int(x) for x in a.sizes.split(",")]

    if a.action == "plan":
        do_plan(sizes)
    elif a.action == "build":
        do_build(sizes, a.only, Path(a.src_dir) if a.src_dir else None)
    elif a.action == "test":
        do_test(sizes, a.only)
    else:
        do_restore()


if __name__ == "__main__":
    main()

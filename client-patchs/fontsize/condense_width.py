#!/usr/bin/env python
"""condense_width.py —— 只压缩字宽（合成窄体），不动字形来源与 hinting。

背景
----
汉化包字体（宋体基）本身没问题，只是中文字太宽把 UI 撑爆。本脚本把字形轮廓
沿 x 轴压缩 ``factor``、同步压缩 advance，字高/字号完全不变。

关键约束（实测得出，勿省）
--------------------------
1. 只压 ``hmtx`` 而不压轮廓不行：CJK 字形墨迹已接近填满字身框，压 advance 必然
   让相邻字相交。必须轮廓与 advance 一起压。
2. 原字体带 12~17px 的 EBDT/EBLC 点阵。**若不重渲点阵，压缩在 12~17px 完全
   失效**（引擎直接用旧位图，实测宽度 159px 一点没变）。所以压完必须按
   压窄后的字形重新光栅化点阵。
3. hinting 不会把宽度"拉回"：实测压 0.9 后开/关 hinting 渲染宽度一致（差 ≤1px），
   因此 fpgm/prep/cvt 可原样保留，避免动 hinting 带来的副作用。

用法
----
    python condense_width.py status                       # 看当前宽度状态
    python condense_width.py apply [--factor 0.9] [--only X.otf]
    python condense_width.py test   [--only X.otf]
    python condense_width.py restore                      # 回到压缩前

``apply`` 是幂等的：先确保工作目录回到"压缩前"的备份状态，再压。所以想换
比例直接重跑 apply --factor 0.85 即可，不会叠加压缩。
"""

from __future__ import annotations

import argparse
import os
import shutil
import sys
import tempfile
import time
from pathlib import Path

try:
    from fontTools.ttLib import TTFont, newTable
    from fontTools.pens.ttGlyphPen import TTGlyphPen
    from fontTools.pens.transformPen import TransformPen
except ImportError:
    sys.exit("缺少 fontTools：python -m pip install fonttools")

sys.path.insert(0, str(Path(__file__).parent))
import build_cjk_font as bcf  # noqa: E402
from add_bitmap_strikes import StrikeRenderer, build_strike  # noqa: E402

BACKUP_GLOB = "Fonts.bak-condense-*"
TMP_DIRNAME = ".condense-tmp"
# 这些表记录的是压缩前的像素级度量，留着会与压缩后的字形打架
STALE_METRIC_TABLES = ("VDMX", "hdmx", "LTSH")


# ------------------------------------------------------------------ helpers
# 注意：目录一律通过 bcf.FONTS_DIR 读取。模块级常量缓存会在 `--fonts` 传入后
# 失效（第二次踩同一个坑），所以这里全部写成函数。

def stamp() -> str:
    return time.strftime("%Y%m%d-%H%M%S")


def fonts_dir() -> Path:
    return Path(bcf.FONTS_DIR)


def backup_root() -> Path:
    return fonts_dir().parent


def font_targets(only: list[str] | None = None) -> list[Path]:
    return list(bcf.font_targets(only))


def latest_backup() -> Path | None:
    baks = sorted(backup_root().glob(BACKUP_GLOB))
    return baks[-1] if baks else None


def cjk_advance(tt: TTFont) -> tuple[int, int]:
    """返回 (CJK advance 众数值, upem)。"""
    upem = tt["head"].unitsPerEm
    cmap = tt.getBestCmap()
    hmtx = tt["hmtx"]
    vals: dict[int, int] = {}
    for cp, gname in cmap.items():
        if 0x4E00 <= cp <= 0x9FFF:
            a = hmtx[gname][0]
            vals[a] = vals.get(a, 0) + 1
    if not vals:
        return 0, upem
    return max(vals.items(), key=lambda kv: kv[1])[0], upem


# ------------------------------------------------------------------ condense

def condense_tt(tt: TTFont, factor: float) -> None:
    """字形轮廓 + advance 一起横向压缩，字高不变。"""
    glyph_set = tt.getGlyphSet()
    glyf = tt["glyf"]
    order = tt.getGlyphOrder()

    baked: dict[str, object] = {}
    for name in order:
        pen = TTGlyphPen(glyph_set)
        glyph_set[name].draw(TransformPen(pen, (factor, 0, 0, 1, 0, 0)))
        baked[name] = pen.glyph()
    for name, glyph in baked.items():
        glyf[name] = glyph

    hmtx = tt["hmtx"]
    for name in order:
        adv, lsb = hmtx[name]
        hmtx[name] = (round(adv * factor), round(lsb * factor))

    hhea = tt["hhea"]
    hhea.advanceWidthMax = round(hhea.advanceWidthMax * factor)
    os2 = tt["OS/2"]
    if getattr(os2, "xAvgCharWidth", 0):
        os2.xAvgCharWidth = round(os2.xAvgCharWidth * factor)

    for tag in STALE_METRIC_TABLES:
        if tag in tt:
            del tt[tag]


def process(path: Path, sizes: list[int], factor: float, tmp_dir: Path):
    """压窄单个字体并重渲点阵，返回 (原大小, 新大小)。"""
    tt = TTFont(path)
    if not sizes:  # 沿用原字体的点阵档位
        sizes = [s.bitmapSizeTable.ppemX for s in tt["EBLC"].strikes] \
            if "EBLC" in tt else [12, 13, 14, 15, 16, 17]

    before, upem = cjk_advance(tt)
    condense_tt(tt, factor)
    for tag in ("EBDT", "EBLC"):  # 旧点阵是旧宽度，必须丢掉重渲
        if tag in tt:
            del tt[tag]

    # 压窄后的无点阵副本：位图必须从这个渲染，否则宽度不一致
    tmp_dir.mkdir(parents=True, exist_ok=True)
    tmp_src = tmp_dir / f"{path.stem}.src{path.suffix}"
    tt.save(tmp_src)

    renderers = {p: StrikeRenderer(tmp_src, None, p) for p in sizes}
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

    old_size = path.stat().st_size
    tmp_out = tmp_dir / f"{path.stem}.out{path.suffix}"
    tt.save(tmp_out)
    os.replace(tmp_out, path)
    new_size = path.stat().st_size
    tt.close()
    tmp_src.unlink(missing_ok=True)
    return before, upem, old_size, new_size, sizes


# ------------------------------------------------------------------ actions

def do_status(only: list[str] | None) -> None:
    print(f"目标目录: {fonts_dir()}")
    print(f"压缩前备份: {latest_backup() or '（无）'}\n")
    print(f"{'文件':<32} {'upem':>5} {'CJK advance':>12} {'宽度比':>7}  点阵档位")
    for p in font_targets(only):
        tt = TTFont(p, lazy=True)
        adv, upem = cjk_advance(tt)
        ppems = "-"
        if "EBLC" in tt:
            ppems = ",".join(str(s.bitmapSizeTable.ppemX) for s in tt["EBLC"].strikes)
        tt.close()
        ratio = adv / upem if upem else 0
        print(f"{p.name:<32} {upem:>5} {adv:>12} {ratio:>7.3f}  {ppems}")


def do_apply(factor: float, only: list[str] | None) -> None:
    bak = latest_backup()
    if bak is None:
        bak = backup_root() / f"Fonts.bak-condense-{stamp()}"
        print(f"备份原始字体 -> {bak.name}")
        bak.mkdir(parents=True, exist_ok=True)
        for p in font_targets(None):
            shutil.copy2(p, bak / p.name)
    else:
        # 幂等：先把工作目录还原到压缩前，避免重复压缩
        print(f"从备份还原基点 -> {bak.name}")
        for p in font_targets(only):
            src = bak / p.name
            if src.exists():
                shutil.copy2(src, p)

    # 临时目录放系统 temp：游戏目录里的 shutil.rmtree 会被环境的 safe-delete
    # 拦截（trash 操作失败），而且不该在 Fonts 下留垃圾。
    tmp_dir = Path(tempfile.mkdtemp(prefix="dfo-condense-"))
    targets = font_targets(only)
    print(f"压缩系数 factor={factor}，共 {len(targets)} 个字体\n")
    total_before = total_after = 0
    for i, p in enumerate(targets, 1):
        t0 = time.time()
        before_adv, upem, old_sz, new_sz, sizes = process(p, [], factor, tmp_dir)
        after_adv, _ = cjk_advance(TTFont(p, lazy=True))
        total_before += old_sz
        total_after += new_sz
        print(f"[{i:>2}/{len(targets)}] {p.name:<32} advance {before_adv}->{after_adv} "
              f"({after_adv / upem:.3f}em)  {old_sz / 1e6:.1f}->{new_sz / 1e6:.1f}MB  "
              f"{time.time() - t0:.0f}s", flush=True)
    try:
        shutil.rmtree(tmp_dir, ignore_errors=True)
    except Exception:
        pass  # 系统 temp 里的残留无所谓
    print(f"\n完成：点阵档位 {sizes}，总体积 {total_before/1e6:.1f} -> {total_after/1e6:.1f}MB")


def do_restore() -> None:
    bak = latest_backup()
    if bak is None:
        sys.exit("没有找到 Fonts.bak-condense-* 备份")
    print(f"从 {bak.name} 还原…")
    for p in font_targets(None):
        src = bak / p.name
        if src.exists():
            shutil.copy2(src, p)
            print(f"  还原 {p.name}")
    print("完成")


def do_test(only: list[str] | None) -> None:
    """抽查：点阵命中（灰阶法）+ 文本宽度对比。"""
    from PIL import Image, ImageDraw, ImageFont

    bak = latest_backup()
    if bak is None:
        sys.exit("没有备份，无法对比")
    text = "技能熟练度消耗法力值"
    print(f"{'文件':<32} 字号  原宽 -> 现宽  比值   点阵命中")
    for p in font_targets(only):
        ref = bak / p.name
        if not ref.exists():
            continue
        tt = TTFont(p, lazy=True)
        if "EBLC" not in tt:
            print(f"{p.name:<32} 无点阵表")
            tt.close()
            continue
        ppems = [s.bitmapSizeTable.ppemX for s in tt["EBLC"].strikes]
        tt.close()

        def render(path, ppem):
            pf = ImageFont.truetype(str(path), size=ppem)
            img = Image.new("L", (420, 32), 0)
            ImageDraw.Draw(img).text((2, 4), text, font=pf, fill=255)
            bb = img.getbbox()
            return (bb[2] - bb[0]) if bb else 0

        for ppem in (ppems[0], ppems[len(ppems) // 2], ppems[-1]):
            w_old = render(ref, ppem)
            w_new = render(p, ppem)
            # 点阵命中判别：渲染结果只有 0/255 两个值 = 命中点阵
            pf = ImageFont.truetype(str(p), size=ppem)
            img = Image.new("L", (64, 32), 0)
            ImageDraw.Draw(img).text((2, 4), "技能", font=pf, fill=255)
            levels = len(set(img.getdata()) - {0})
            hit = "是" if levels <= 2 else f"否({levels}灰阶)"
            print(f"{p.name if ppem == ppems[0] else '':<32} {ppem:>4}px "
                  f"{w_old:>4} -> {w_new:>4}  {w_new / w_old:.3f}   {hit}")


# ------------------------------------------------------------------ main

def main() -> None:
    ap = argparse.ArgumentParser(description="只压缩字体宽度（合成窄体）")
    ap.add_argument("action", choices=("status", "apply", "test", "restore"))
    ap.add_argument("--factor", type=float, default=0.9, help="横向压缩系数，默认 0.9")
    ap.add_argument("--only", nargs="*", help="只处理指定文件名")
    ap.add_argument("--fonts", help="指定 Fonts 目录")
    a = ap.parse_args()

    if a.fonts:
        bcf.FONTS_DIR = Path(a.fonts)

    if a.action == "status":
        do_status(a.only)
    elif a.action == "apply":
        do_apply(a.factor, a.only)
    elif a.action == "test":
        do_test(a.only)
    elif a.action == "restore":
        do_restore()


if __name__ == "__main__":
    main()

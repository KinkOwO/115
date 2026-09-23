#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
DFO 115US 客户端「字号缩放」补丁调整工具（可回滚）

背景
----
客户端目录里的 `补丁说明.md` 记录了 2026-09-18 打进 DFO.exe 的两处改动：

  1. 字号放大 K=1.1：在 .text 尾部 slack（码洞 VA 0x14918563F）注入
         movss xmm6, [rbx+0A8h]
         mulss xmm6, [rip+...]      ; 乘以 K
         jmp   回到原指令流
     并把 VA 0x147044451 / 0x147044465 两处「字号读点」改成 jmp 跳到码洞。
  2. 字号钳制放开：上限常量 96 -> 384（VA 0x14DC6C270），下限 8 不变。

K=1.1 会让所有文字比官方大 10%，UI 文本框宽度是按官方字号定的，
于是很多汉化文本（中文比英文宽）显示不完整。

本工具只改这 3 个常量，不改任何逻辑，且每次改前自动备份 DFO.exe。

用法
----
  python patch_font_scale.py status                 # 查看当前值
  python patch_font_scale.py set-scale 1.0          # 改字号倍数（还原官方）
  python patch_font_scale.py set-scale 1.05         # 轻微放大
  python patch_font_scale.py set-clamp 96           # 还原官方上限（可选）
  python patch_font_scale.py restore                # 从备份还原

注意：游戏运行时 DFO.exe 被占用，必须先退出游戏再执行。
"""

from __future__ import annotations

import argparse
import datetime as _dt
import glob
import os
import struct
import sys

IMAGE_BASE = 0x140000000

# 码洞里两段注入代码（用于校验补丁确实在位）
# 两段都由「P1/P2 字号读点」的 jmp 跳进来，段尾再 jmp 回原指令流
CAVE_VAS = (0x14918563F, 0x14918567F)
# 两处被改写成 jmp 的「字号读点」
HOOK_VAS = (0x147044451, 0x147044465)

# 两个 1.1 常量（第二个是真正被 mulss 引用的，第一个也一起改，保持一致）
SCALE_VAS = (0x1491856BF, 0x1491856C3)

CLAMP_MAX_VA = 0x14DC6C270   # 官方 96，补丁后 384
CLAMP_MIN_VA = 0x14DC6C274   # 官方 8，补丁未改

DEFAULT_CLIENT = r"C:\Game\dof\115us\DFO"


class Client:
    def __init__(self, path: str, write: bool):
        self.path = path
        self.mode = "r+b" if write else "rb"
        self.f = open(path, self.mode)
        self._check_layout()

    def close(self):
        self.f.close()

    # ---- PE ----
    def _sections(self):
        f = self.f
        f.seek(0x3C)
        pe = struct.unpack("<I", f.read(4))[0]
        f.seek(pe)
        if f.read(4) != b"PE\0\0":
            raise RuntimeError("不是有效的 PE 文件")
        f.seek(pe + 6)
        nsec = struct.unpack("<H", f.read(2))[0]
        f.seek(pe + 20)
        size_opt = struct.unpack("<H", f.read(2))[0]
        base = pe + 24 + size_opt
        out = []
        for i in range(nsec):
            f.seek(base + i * 40)
            d = f.read(40)
            name = d[:8].rstrip(b"\x00").decode("latin1", "replace")
            vsize, va, rsize, raw = struct.unpack("<IIII", d[8:24])
            out.append((name, va, vsize, raw, rsize))
        return out

    def _check_layout(self):
        """本工具按「文件偏移 == RVA」定位。验证关键节满足该关系。"""
        for name, va, vsize, raw, rsize in self._sections():
            if name in (".text", ".data") and raw != va:
                raise RuntimeError(
                    f"节 {name} 的 RawOff({raw:#x}) != VA({va:#x})，"
                    "地址换算假设不成立，拒绝继续。"
                )

    # ---- primitives ----
    def read(self, va: int, n: int) -> bytes:
        self.f.seek(va - IMAGE_BASE)
        return self.f.read(n)

    def write(self, va: int, data: bytes) -> None:
        self.f.seek(va - IMAGE_BASE)
        self.f.write(data)

    def f32(self, va: int) -> float:
        return struct.unpack("<f", self.read(va, 4))[0]

    def i32(self, va: int) -> int:
        return struct.unpack("<i", self.read(va, 4))[0]


def hooks_present(c: Client) -> bool:
    """校验两处「字号读点」确实是 jmp 到码洞，且码洞里是 mulss 缩放。"""
    for va in HOOK_VAS:
        b = c.read(va, 5)
        if b[0] != 0xE9:
            return False
        rel = struct.unpack("<i", b[1:5])[0]
        if va + 5 + rel not in CAVE_VAS:
            return False
    for va in CAVE_VAS:
        # f3 0f 10 b3 a8 00 00 00   movss xmm6,[rbx+0A8h]
        # f3 0f 59 35 xx xx xx xx   mulss xmm6,[rip+disp32]
        if c.read(va, 8) != b"\xf3\x0f\x10\xb3\xa8\x00\x00\x00":
            return False
        if c.read(va + 8, 3) != b"\xf3\x0f\x59":
            return False
    return True


def backup(path: str) -> str:
    stamp = _dt.datetime.now().strftime("%Y%m%d-%H%M%S")
    dst = f"{path}.bak-fontscale-{stamp}"
    if os.path.exists(dst):
        return dst
    with open(path, "rb") as src, open(dst, "wb") as out:
        while True:
            chunk = src.read(8 << 20)
            if not chunk:
                break
            out.write(chunk)
    return dst


def latest_backup(path: str) -> str | None:
    cands = sorted(glob.glob(path + ".bak-fontscale-*"))
    return cands[-1] if cands else None


def cmd_status(c: Client, path: str) -> int:
    print(f"客户端: {path}")
    print(f"钩子在位: {'是' if hooks_present(c) else '否（未被 09-18 补丁修改过，或已被还原）'}")
    for va in SCALE_VAS:
        print(f"  字号倍数常量 VA {va:#x} = {c.f32(va):.6f}")
    print(f"  字号上限 VA {CLAMP_MAX_VA:#x} = {c.i32(CLAMP_MAX_VA)}  (官方 96)")
    print(f"  字号下限 VA {CLAMP_MIN_VA:#x} = {c.i32(CLAMP_MIN_VA)}  (官方 8)")
    b = latest_backup(path)
    print(f"  最近备份: {b or '无'}")
    return 0


def cmd_set_scale(c: Client, path: str, k: float) -> int:
    if not hooks_present(c):
        print("错误: 未检测到 09-18 字号补丁的钩子，拒绝修改。", file=sys.stderr)
        return 2
    if not 0.5 <= k <= 2.0:
        print("错误: K 超出合理范围 [0.5, 2.0]。", file=sys.stderr)
        return 2
    old = [c.f32(v) for v in SCALE_VAS]
    dst = backup(path)
    data = struct.pack("<f", k)
    for va in SCALE_VAS:
        c.write(va, data)
    c.f.flush()
    print(f"已备份: {dst}")
    print(f"字号倍数 {old[0]:.3f} -> {k:.3f}（VA {SCALE_VAS[0]:#x} / {SCALE_VAS[1]:#x}）")
    print("重启游戏生效。若不满意可执行 restore 或再次 set-scale。")
    return 0


def cmd_set_clamp(c: Client, path: str, value: int) -> int:
    old = c.i32(CLAMP_MAX_VA)
    dst = backup(path)
    c.write(CLAMP_MAX_VA, struct.pack("<i", value))
    c.f.flush()
    print(f"已备份: {dst}")
    print(f"字号上限 {old} -> {value}")
    return 0


def cmd_restore(path: str) -> int:
    src = latest_backup(path)
    if not src:
        print("没有找到备份，无法还原。", file=sys.stderr)
        return 1
    try:
        with open(path, "r+b") as out:
            with open(src, "rb") as f:
                while True:
                    chunk = f.read(8 << 20)
                    if not chunk:
                        break
                    out.write(chunk)
            out.truncate()
    except PermissionError:
        print("错误: DFO.exe 被占用，请先完全退出游戏再还原。", file=sys.stderr)
        return 3
    print(f"已从 {src} 还原 {path}")
    return 0


def main() -> int:
    ap = argparse.ArgumentParser(description="DFO 115US 字号缩放补丁调整（可回滚）")
    ap.add_argument("action", choices=["status", "set-scale", "set-clamp", "restore"])
    ap.add_argument("value", nargs="?")
    ap.add_argument("--dir", default=os.environ.get("DFO_CLIENT_DIR", DEFAULT_CLIENT))
    args = ap.parse_args()

    path = os.path.join(args.dir, "DFO.exe")
    if not os.path.exists(path):
        print(f"找不到 {path}，用 --dir 指定客户端目录。", file=sys.stderr)
        return 1

    if args.action == "restore":
        return cmd_restore(path)

    write = args.action in ("set-scale", "set-clamp")
    try:
        c = Client(path, write)
    except PermissionError:
        print("错误: DFO.exe 被占用，请先完全退出游戏（含 CEF.exe）再执行。", file=sys.stderr)
        return 3
    try:
        if args.action == "status":
            return cmd_status(c, path)
        if args.action == "set-scale":
            if args.value is None:
                print("需要给出倍数，例如 set-scale 1.0", file=sys.stderr)
                return 2
            return cmd_set_scale(c, path, float(args.value))
        if args.action == "set-clamp":
            if args.value is None:
                print("需要给出上限，例如 set-clamp 96", file=sys.stderr)
                return 2
            return cmd_set_clamp(c, path, int(args.value))
    finally:
        c.close()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

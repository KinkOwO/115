#!/usr/bin/env python3
"""把 client-patchs/chinese-input 的产物打成 schema 2 四层 mod 包。

包结构：
    chinese-input-<版本>.zip
      mod.json                 组织文件（根）
      README.md
      client/ChineseLocalization.dll

DLL 由 build-probe.cmd 用 MSVC 构建；本脚本只负责算 size/sha256 并打包，
所以在开发机上不需要 Go/Python 之外的依赖。
"""

import hashlib
import json
import os
import sys
import zipfile

HERE = os.path.dirname(os.path.abspath(__file__))
DIST = os.path.join(HERE, "dist")
DLL = os.path.join(DIST, "ChineseLocalization.dll")
STAGE = os.path.join(DIST, "staging")
README = os.path.join(HERE, "PROBE-README.md")
INI = os.path.join(HERE, "chinese-input.ini")

MOD_ID = "qol.chinese-input-probe"
VERSION = "2.0.3"
# 插件形态：落位到宿主的插件目录，由 qol.client-host 加载。
PLUGIN_DIR = ".115us-mods"
PLUGIN_NAME = "ChineseInputProbe.dll"
INI_NAME = "chinese-input.ini"


def sha256_of(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def main():
    if not os.path.isfile(DLL):
        print("FAIL: 先跑 build-probe.cmd 生成 dist/ChineseLocalization.dll", file=sys.stderr)
        return 1
    if not os.path.isfile(INI):
        print("FAIL: 缺少 chinese-input.ini（插件开关）", file=sys.stderr)
        return 1
    size = os.path.getsize(DLL)
    digest = sha256_of(DLL)
    ini_size = os.path.getsize(INI)
    ini_digest = sha256_of(INI)

    manifest = {
        "schema": 2,
        "id": MOD_ID,
        "version": VERSION,
        "name": "中文输出支持",
        "author": "DFO 115us",
        "description": (
            "让这个客户端能打中文。① 修正 Themida 导入槽，接管它对 user32/imm32 的调用；"
            "② 把「只认 XP 时代传统 IME」的门禁喂过去（键盘布局 HKL、ImmGetIMEFileNameW 文件名、"
            "ImmIsIME）；③ 吞掉它每次按键的取消组字（ImmNotifyIME/CPS_CANCEL），并在它不接收"
            "上屏文字时兜底投 WM_CHAR。只对游戏主模块生效，开关在 chinese-input.ini。"
        ),
        "permissions": ["client.file.write"],
        "requires": ["qol.client-host"],
        "layers": {
            "client": {
                "ops": [
                    {
                        "kind": "file.add",
                        "target": "%s/%s" % (PLUGIN_DIR, PLUGIN_NAME),
                        "source": "client/" + PLUGIN_NAME,
                        "size": size,
                        "sha256": digest,
                        "note": (
                            "客户端唯一可自动加载的槽位由 qol.client-host 占用，本探针作为它的插件运行。"
                            "开关文件 chinese-input.ini 由插件首次运行时自己生成（不随包落位），"
                            "这样升级只替换这一个文件，不会撞上 file.add 不能覆盖已改文件的限制。"
                        ),
                    }
                ]
            }
        },
    }

    os.makedirs(STAGE, exist_ok=True)
    os.makedirs(os.path.join(STAGE, "client"), exist_ok=True)
    with open(os.path.join(STAGE, "mod.json"), "w", encoding="utf-8", newline="\n") as f:
        json.dump(manifest, f, ensure_ascii=False, indent=2)
        f.write("\n")
    with open(os.path.join(STAGE, "client", PLUGIN_NAME), "wb") as f:
        f.write(open(DLL, "rb").read())
    with open(os.path.join(STAGE, "client", INI_NAME), "wb") as f:
        f.write(open(INI, "rb").read())
    if os.path.isfile(README):
        with open(os.path.join(STAGE, "README.md"), "wb") as f:
            f.write(open(README, "rb").read())

    out = os.path.join(DIST, "chinese-input-%s.zip" % VERSION)
    with zipfile.ZipFile(out, "w", zipfile.ZIP_DEFLATED) as z:
        z.write(os.path.join(STAGE, "mod.json"), "mod.json")
        z.write(os.path.join(STAGE, "client", PLUGIN_NAME), "client/" + PLUGIN_NAME)
        z.write(os.path.join(STAGE, "client", INI_NAME), "client/" + INI_NAME)
        if os.path.isfile(os.path.join(STAGE, "README.md")):
            z.write(os.path.join(STAGE, "README.md"), "README.md")

    print("mod     : %s" % MOD_ID)
    print("dll     : %s (%d bytes, sha256 %s)" % (DLL, size, digest))
    print("ini     : %s (%d bytes, sha256 %s)" % (INI, ini_size, ini_digest))
    print("zip     : %s (%d bytes)" % (out, os.path.getsize(out)))
    return 0


if __name__ == "__main__":
    sys.exit(main())

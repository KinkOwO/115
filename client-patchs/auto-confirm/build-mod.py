#!/usr/bin/env python3
r"""把 client-patchs/auto-confirm 的产物打成 schema 2 四层 mod 包。

包结构：
    auto-confirm-<版本>.zip
      mod.json
      README.md
      client/AutoConfirmDelete.dll   ← 落位成 <客户端>\.115us-mods\AutoConfirmDelete.dll
"""

import hashlib
import json
import os
import sys
import zipfile

HERE = os.path.dirname(os.path.abspath(__file__))
DIST = os.path.join(HERE, "dist")
DLL = os.path.join(DIST, "AutoConfirmDelete.dll")
STAGE = os.path.join(DIST, "staging")
README = os.path.join(HERE, "AUTO-CONFIRM-README.md")

MOD_ID = "qol.auto-confirm"
VERSION = "1.0.0"
PLUGIN_DIR = ".115us-mods"
PLUGIN_NAME = "AutoConfirmDelete.dll"


def sha256_of(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def main():
    if not os.path.isfile(DLL):
        print("FAIL: 先跑 build-plugin.cmd 生成 dist/AutoConfirmDelete.dll", file=sys.stderr)
        return 1
    size = os.path.getsize(DLL)
    digest = sha256_of(DLL)

    manifest = {
        "schema": 2,
        "id": MOD_ID,
        "version": VERSION,
        "name": "自动确认（删角色）",
        "author": "DFO 115us",
        "description": (
            "去掉『删角色要先手打确认短语』这一步：把删角色通知窗里那个编辑框的"
            "『取文本』槽换成包装，只对该实例返回客户端自己的确认短语（DSTR 100086308）。"
            "名字由客户端自己提供，不重新实现协议；仍由玩家点一次『确定』。"
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
                            "作为 qol.client-host 的插件运行；开关 auto-confirm.ini 由插件首次"
                            "运行时自己生成。卸载即逐字节删除。"
                        ),
                    }
                ]
            }
        },
    }

    os.makedirs(os.path.join(STAGE, "client"), exist_ok=True)
    with open(os.path.join(STAGE, "mod.json"), "w", encoding="utf-8", newline="\n") as f:
        json.dump(manifest, f, ensure_ascii=False, indent=2)
        f.write("\n")
    with open(os.path.join(STAGE, "client", PLUGIN_NAME), "wb") as f:
        f.write(open(DLL, "rb").read())
    if os.path.isfile(README):
        with open(os.path.join(STAGE, "README.md"), "wb") as f:
            f.write(open(README, "rb").read())

    out = os.path.join(DIST, "auto-confirm-%s.zip" % VERSION)
    with zipfile.ZipFile(out, "w", zipfile.ZIP_DEFLATED) as z:
        z.write(os.path.join(STAGE, "mod.json"), "mod.json")
        z.write(os.path.join(STAGE, "client", PLUGIN_NAME), "client/" + PLUGIN_NAME)
        if os.path.isfile(os.path.join(STAGE, "README.md")):
            z.write(os.path.join(STAGE, "README.md"), "README.md")

    print("mod     : %s" % MOD_ID)
    print("dll     : %s (%d bytes, sha256 %s)" % (DLL, size, digest))
    print("zip     : %s (%d bytes)" % (out, os.path.getsize(out)))
    return 0


if __name__ == "__main__":
    sys.exit(main())

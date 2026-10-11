#!/usr/bin/env python3
r"""把 client-patchs/client-host 的产物打成 schema 2 四层 mod 包。

包结构：
    client-host-<版本>.zip
      mod.json                 组织文件（根）
      README.md
      client/ChineseLocalization.dll     ← 落位成 <客户端>\ChineseLocalization.dll

这一个文件是客户端**唯一**能被自动加载的槽位（见 HOST-README），
其它客户端 mod 都通过它放进 <客户端>\.115us-mods\ 里的插件 DLL 生效。
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
README = os.path.join(HERE, "HOST-README.md")

MOD_ID = "qol.client-host"
VERSION = "1.0.0"


def sha256_of(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def main():
    if not os.path.isfile(DLL):
        print("FAIL: 先跑 build-host.cmd 生成 dist/ChineseLocalization.dll", file=sys.stderr)
        return 1
    size = os.path.getsize(DLL)
    digest = sha256_of(DLL)

    manifest = {
        "schema": 2,
        "id": MOD_ID,
        "version": VERSION,
        "name": "客户端DLL框架",
        "author": "DFO 115us",
        "description": (
            "把客户端唯一可自动加载的 DLL 槽位做成插件目录：加载并启动 "
            "<客户端>\\.115us-mods\\*.dll 里的客户端 mod 插件。"
            "自身不改游戏行为；日志写在客户端根 client-host.log。"
        ),
        "permissions": ["client.file.write"],
        "layers": {
            "client": {
                "ops": [
                    {
                        "kind": "file.add",
                        "target": "ChineseLocalization.dll",
                        "source": "client/ChineseLocalization.dll",
                        "size": size,
                        "sha256": digest,
                        "note": (
                            "客户端自带的 dinput8.dll 代理会自动加载本文件并调用 StartLocalization；"
                            "插件从 <客户端>\\.115us-mods\\ 读。卸载即逐字节删除。"
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
    with open(os.path.join(STAGE, "client", "ChineseLocalization.dll"), "wb") as f:
        f.write(open(DLL, "rb").read())
    if os.path.isfile(README):
        with open(os.path.join(STAGE, "README.md"), "wb") as f:
            f.write(open(README, "rb").read())

    out = os.path.join(DIST, "client-host-%s.zip" % VERSION)
    with zipfile.ZipFile(out, "w", zipfile.ZIP_DEFLATED) as z:
        z.write(os.path.join(STAGE, "mod.json"), "mod.json")
        z.write(os.path.join(STAGE, "client", "ChineseLocalization.dll"),
                "client/ChineseLocalization.dll")
        if os.path.isfile(os.path.join(STAGE, "README.md")):
            z.write(os.path.join(STAGE, "README.md"), "README.md")

    print("mod     : %s" % MOD_ID)
    print("dll     : %s (%d bytes)" % (DLL, size))
    print("sha256  : %s" % digest)
    print("zip     : %s (%d bytes)" % (out, os.path.getsize(out)))
    return 0


if __name__ == "__main__":
    sys.exit(main())

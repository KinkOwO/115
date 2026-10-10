#!/usr/bin/env python3
"""把 client-patchs/difficulty 的产物打成 schema 2 四层 mod 包。

包结构：
    difficulty-<版本>.zip
      mod.json                 组织文件（根）
      README.md
      client/DifficultyRules.dll

包里**只有 DLL**：规则文件 rules.json 由插件首次运行时自己在
<客户端>\\.115us-mods\\ 里生成（modkit 的 file.add 不允许覆盖
"已存在且内容不同"的文件，随包落位会让升级撞墙）。

DLL 由 build-dll.cmd 用 MSVC 构建；本脚本只负责算 size/sha256 并打包。
"""

import hashlib
import json
import os
import sys
import zipfile

HERE = os.path.dirname(os.path.abspath(__file__))
DIST = os.path.join(HERE, "dist")
DLL = os.path.join(DIST, "DifficultyRules.dll")
STAGE = os.path.join(DIST, "staging")
README = os.path.join(HERE, "README.md")

MOD_ID = "difficulty.rules"
VERSION = "1.0.0"
PLUGIN_DIR = ".115us-mods"
PLUGIN_NAME = "DifficultyRules.dll"


def sha256_of(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def main():
    if not os.path.isfile(DLL):
        print("FAIL: 先跑 build-dll.cmd 生成 dist/DifficultyRules.dll", file=sys.stderr)
        return 1
    size = os.path.getsize(DLL)
    digest = sha256_of(DLL)

    manifest = {
        "schema": 2,
        "id": MOD_ID,
        "version": VERSION,
        "name": "副本难度（怪物血量 + 怪物伤害倍率）",
        "author": "DFO 115us",
        "description": (
            "把启动器内嵌 GM 的「副本难度」字段执行逻辑搬进客户端进程：按 rules.json 里"
            "命中的副本，把怪物的四层血量基础值与当前血量按 percent 缩放（保住已受伤比例），"
            "并把物攻(desc+0x398)/魔攻(desc+0x3b8)按 attackPercent 缩放（受保护 32 位原生整数，"
            "逐字节核对 stored+guard）。attackPercent 省略时默认 100（不改伤害）。"
            "不改 DFO.exe、不改任何函数字节、不落位 dinput8.dll；"
            "写前逐字节核对现场，对不上就跳过并记日志。默认 enabled=false（不偷偷加强）。"
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
                            "客户端唯一可自动加载的槽位由 qol.client-host 占用，本插件作为它的插件运行。"
                            "规则文件 rules.json 由插件首次运行时自己生成（不随包落位），"
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
    if os.path.isfile(README):
        with open(os.path.join(STAGE, "README.md"), "wb") as f:
            f.write(open(README, "rb").read())

    out = os.path.join(DIST, "difficulty-%s.zip" % VERSION)
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

# -*- coding: utf-8 -*-
"""把本 mod 打成 zip（正式分发形态）。

只装这些东西：
    mod.json          组织文件（引擎按它读 server.scripts）
    README.md         给打包的人看
    server/rules/*.lua 两个规则脚本

自带两条自检，避免"包打出来了但装不上"：
  1. mod.json 能解析、schema == 2、id/version/name 齐全；
  2. 清单里 server.scripts 声明的每一份 .lua 都真的在包里。

用法：python build-mod.py
产物：dist/<id>-<version>.zip
"""
import json
import os
import sys
import zipfile

HERE = os.path.dirname(os.path.abspath(__file__))
DIST = os.path.join(HERE, "dist")

INCLUDE = ["mod.json", "README.md", "server"]


def fail(msg):
    print("打包失败：%s" % msg)
    sys.exit(1)


def main():
    man_path = os.path.join(HERE, "mod.json")
    with open(man_path, encoding="utf-8") as f:
        man = json.load(f)

    if man.get("schema") != 2:
        fail("mod.json 的 schema 必须是 2（当前 %r）" % man.get("schema"))
    for key in ("id", "version", "name"):
        if not str(man.get(key) or "").strip():
            fail("mod.json 缺少 %s" % key)

    scripts = ((man.get("layers") or {}).get("server") or {}).get("scripts") or []
    if not scripts:
        fail("mod.json 的 layers.server.scripts 是空的：这个 mod 就靠它们干活")
    for rel in scripts:
        if not os.path.isfile(os.path.join(HERE, rel.replace("/", os.sep))):
            fail("清单声明的脚本不在包里：%s" % rel)

    os.makedirs(DIST, exist_ok=True)
    out = os.path.join(DIST, "%s-%s.zip" % (man["id"], man["version"]))

    files = []
    for item in INCLUDE:
        full = os.path.join(HERE, item)
        if os.path.isfile(full):
            files.append(item)
        elif os.path.isdir(full):
            for root, _dirs, names in os.walk(full):
                for n in sorted(names):
                    p = os.path.join(root, n)
                    files.append(os.path.relpath(p, HERE).replace(os.sep, "/"))
        else:
            fail("要打进包的东西不见了：%s" % item)

    with zipfile.ZipFile(out, "w", zipfile.ZIP_DEFLATED) as z:
        for rel in sorted(set(files)):
            z.write(os.path.join(HERE, rel.replace("/", os.sep)), rel)

    size = os.path.getsize(out)
    print("已生成：%s" % out)
    print("  %d 个条目 / %d 字节" % (len(set(files)), size))
    print("  脚本：%s" % "、".join(scripts))


main()

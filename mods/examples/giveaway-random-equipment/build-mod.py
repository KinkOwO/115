#!/usr/bin/env python3
"""打包 giveaway-random-equipment（新角色随机送装备）。

与 hello-verify 的打包脚本同一套路：收集 server 层 → 算 sha256 → 生成组织文件 → 打包 → 自证。

用法：
  python build-mod.py                        # 输出到 <脚本目录>/dist
  python build-mod.py --modkit <modkit.exe>  # 顺便跑一次 verify

退出码：0 成功；1 失败。
"""

import argparse
import hashlib
import io
import json
import os
import shutil
import subprocess
import sys
import zipfile

MOD_ID = "giveaway.random-equipment"
VERSION = "1.0.0"
ZIP_DATE = (2026, 10, 6, 0, 0, 0)  # 固定时间戳 → 可复现
HERE = os.path.dirname(os.path.abspath(__file__))


def sha256_of(path):
    h = hashlib.sha256()
    with io.open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def read_text(path):
    with io.open(path, "r", encoding="utf-8", errors="replace") as f:
        return f.read()


def stage_tree(src, dst):
    if not os.path.isdir(src):
        return 0
    n = 0
    for root, _dirs, files in os.walk(src):
        for name in files:
            full = os.path.join(root, name)
            rel = os.path.relpath(full, src)
            target = os.path.join(dst, rel)
            os.makedirs(os.path.dirname(target), exist_ok=True)
            shutil.copy2(full, target)
            n += 1
    return n


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--out", default=os.path.join(HERE, "dist"))
    ap.add_argument("--modkit", default=None)
    args = ap.parse_args()

    staging = os.path.join(args.out, "staging")
    if os.path.isdir(staging):
        shutil.rmtree(staging)
    os.makedirs(staging, exist_ok=True)

    # ---- 1) server 层（Go 源码 + 内嵌的 Lua 规则）----
    server_src = os.path.join(HERE, "server")
    if not os.path.isdir(server_src):
        print("FAIL: missing server/ layer", file=sys.stderr)
        return 1
    n_server = stage_tree(server_src, os.path.join(staging, "server"))
    if not os.path.isfile(os.path.join(server_src, "rules", "giveaway.lua")):
        print("FAIL: server/rules/giveaway.lua missing (规则脚本是 mod 的核心)", file=sys.stderr)
        return 1
    print("server layer: %d file(s)（含 rules/giveaway.lua）" % n_server)

    # ---- 2) 从源码推导钩子与权限 ----
    code = read_text(os.path.join(server_src, "mod.go"))
    hooks = []
    if "RegisterRewardScript(" in code:
        hooks.append({"name": "reward.script",
                      "note": "向奖励管线登记新角色随机装备规则"})
    if "RegisterBoot(" in code:
        hooks.append({"name": "server.boot",
                      "note": "启动自检：确认规则脚本已进入奖励管线"})
    if "RegisterConsole(" in code:
        hooks.append({"name": "console.command",
                      "note": "一次性诊断命令：status / pool"})
    if not hooks:
        print("FAIL: server/mod.go 没有登记任何钩子", file=sys.stderr)
        return 1

    permissions = sorted({"server.hook"})

    # ---- 3) 组织文件 ----
    manifest = {
        "schema": 2,
        "id": MOD_ID,
        "version": VERSION,
        "name": "新角色见面礼（金币+邮件）",
        "author": "DFO 115us",
        "description": ("新角色创建时发一笔金币并寄一封系统邮件，用于验证 mod 注册与奖励管线生效。"
                        "规则复用服务端事件奖励管线，幂等不重复发；"
                        "可在 mod 管理器中勾选启用/禁用，改动需重启服务端生效。"),
        "permissions": permissions,
        "layers": {
            "server": {"package": ".", "hooks": hooks},
        },
    }
    manifest_path = os.path.join(staging, "mod.json")
    with io.open(manifest_path, "w", encoding="utf-8", newline="\n") as f:
        f.write(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n")

    # ---- 4) 打包 ----
    zip_path = os.path.join(args.out, "%s-%s.zip" % (MOD_ID, VERSION))
    if os.path.exists(zip_path):
        os.remove(zip_path)

    members = []
    for root, _dirs, files in os.walk(staging):
        for name in files:
            full = os.path.join(root, name)
            rel = os.path.relpath(full, staging).replace(os.sep, "/")
            members.append((rel, full))
    members.sort(key=lambda kv: (kv[0] != "mod.json", kv[0]))

    with zipfile.ZipFile(zip_path, "w", zipfile.ZIP_DEFLATED) as z:
        for rel, full in members:
            zi = zipfile.ZipInfo(rel, date_time=ZIP_DATE)
            zi.compress_type = zipfile.ZIP_DEFLATED
            zi.external_attr = 0o644 << 16
            with io.open(full, "rb") as f:
                z.writestr(zi, f.read())

    print("")
    print("built : %s" % zip_path)
    print("size  : %d bytes" % os.path.getsize(zip_path))
    print("perms : %s" % ", ".join(permissions))
    print("hooks : %s" % ", ".join(h["name"] for h in hooks))

    if args.modkit:
        mk = args.modkit
        if not os.path.isfile(mk):
            print("FAIL: modkit not found: %s" % mk, file=sys.stderr)
            return 1
        print("")
        print("verify:")
        rc = subprocess.call([mk, "verify", "--mod", zip_path])
        if rc != 0:
            print("FAIL: modkit verify exit=%d" % rc, file=sys.stderr)
            return 1

    print("")
    print("next:")
    print("  modkit install --client CLIENTDIR --mod \"%s\" --root LAUNCHERROOT" % zip_path)
    return 0


if __name__ == "__main__":
    sys.exit(main())

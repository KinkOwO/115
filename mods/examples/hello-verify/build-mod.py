#!/usr/bin/env python3
"""把 hello-verify 的源码打成可安装的 mod zip 包（含组织文件 mod.json）。

这个脚本是**打包契约的可执行文档**：任何 mod 都可以照这个套路打包。

它做三件作者每次都要做的事，并且做在明处：

  1) 按四层目录收集交付物；
  2) 为每个"包内资源"算出真实的 size 与 sha256（人手算必错的地方）；
  3) 生成 mod.json 并打出 zip（组织文件在根）。

为什么用 Python 而不是 PowerShell：
  - zip 写入不依赖 .NET 程序集（本机 Windows PowerShell 5.1 是 .NET Core，
    没有 System.IO.Compression.Assembly/FileSystem，ZipFile/ZipArchive 都不可用）；
  - 中文与文件路径按 UTF-8 处理，不受控制台代码页影响；
  - 逻辑可读、可测试。

用法：
  python build-mod.py                  # 输出到 <脚本目录>/dist
  python build-mod.py --out <目录>
  python build-mod.py --modkit <modkit.exe>   # 顺便跑一次 verify 自证

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

MOD_ID = "demo.hello-verify"
VERSION = "1.0.0"
ZIP_DATE = (2026, 10, 6, 0, 0, 0)  # 固定时间戳 → 同样输入产出同样字节

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
    """整目录搬运（保留子目录结构）。"""
    if not os.path.isdir(src):
        return 0
    count = 0
    for root, _dirs, files in os.walk(src):
        for name in files:
            full = os.path.join(root, name)
            rel = os.path.relpath(full, src)
            target = os.path.join(dst, rel)
            os.makedirs(os.path.dirname(target), exist_ok=True)
            shutil.copy2(full, target)
            count += 1
    return count


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--out", default=os.path.join(HERE, "dist"))
    ap.add_argument("--modkit", default=None)
    ap.add_argument("--no-pvf", action="store_true",
                    help="omit the pvf layer (use when pwsh 7 is unavailable)")
    args = ap.parse_args()

    staging = os.path.join(args.out, "staging")
    if os.path.isdir(staging):
        shutil.rmtree(staging)
    os.makedirs(staging, exist_ok=True)

    # ---- 1) 收集四层 ----
    server_src = os.path.join(HERE, "server")
    if not os.path.isdir(server_src):
        print("FAIL: missing server/ layer", file=sys.stderr)
        return 1
    n_server = stage_tree(server_src, os.path.join(staging, "server"))
    print("server layer: %d file(s)" % n_server)

    n_pvf = 0
    pvf_scripts = []
    pvf_dst = os.path.join(staging, "pvf")
    if not args.no_pvf:
        n_pvf = stage_tree(os.path.join(HERE, "pvf"), os.path.join(staging, "pvf"))
    if os.path.isdir(pvf_dst):
        pvf_scripts = sorted(f for f in os.listdir(pvf_dst)
                             if f.lower().endswith(".ps1")
                             and os.path.isfile(os.path.join(pvf_dst, f)))
    print("pvf layer   : %d file(s), %d script(s)" % (n_pvf, len(pvf_scripts)))

    # client 层：整文件资源 → 每个都要 size+sha256
    client_map = {
        "mods-demo-hello-verify.txt": "mods-demo-hello-verify.txt",
    }
    client_ops = []
    if os.path.isdir(os.path.join(HERE, "client")):
        stage_tree(os.path.join(HERE, "client"), os.path.join(staging, "client"))
        for name in sorted(client_map):
            src = os.path.join(staging, "client", name)
            if not os.path.isfile(src):
                print("FAIL: client file declared but missing: %s" % name, file=sys.stderr)
                return 1
            client_ops.append({
                "kind": "file.add",
                "target": client_map[name],
                "source": "client/" + name,
                "size": os.path.getsize(src),
                "sha256": sha256_of(src),
                "note": "demo: harmless marker file; uninstall removes it",
            })
    print("client layer: %d op(s)" % len(client_ops))

    # resource 层：本示例不带资源。作者自备 .img 时在这里列 op。
    resource_dir = os.path.join(HERE, "resource")
    if os.path.isdir(resource_dir) and os.listdir(resource_dir):
        print("FAIL: resource/ has files but no op is declared. 这是刻意失败："
              "打包脚本要求作者显式写出 target/entry，"
              "避免资源悄悄进包却没人知道它挂到哪。", file=sys.stderr)
        return 1

    # ---- 2) 从源码推导 server 钩子（避免清单与代码漂移）----
    mod_go = os.path.join(server_src, "mod.go")
    if not os.path.isfile(mod_go):
        print("FAIL: server/mod.go missing", file=sys.stderr)
        return 1
    code = read_text(mod_go)

    hooks = []
    if "RegisterBoot(" in code:
        hooks.append({"name": "server.boot",
                      "note": "startup self-check + effective config log"})
    if "RegisterConsole(" in code:
        hooks.append({"name": "console.command",
                      "note": "one-shot command via DFO_SERVERMOD_CONSOLE"})
    if "RegisterResponse(" in code:
        hooks.append({"name": "protocol.response",
                      "note": "read-only S2C observer"})
    if not hooks:
        print("FAIL: server/mod.go registers no hook; the mod would do nothing", file=sys.stderr)
        return 1

    permissions = {"server.hook"}
    if client_ops:
        permissions.add("client.file.write")
    if pvf_scripts:
        permissions.update(("pvf.merge", "exec.script"))
    permissions = sorted(permissions)

    # ---- 3) 生成组织文件 ----
    layers = {"server": {"package": ".", "hooks": hooks}}
    if pvf_scripts:
        layers["pvf"] = {"ops": [
            {"kind": "verify", "script": "pvf/" + s, "readOnly": True,
             "args": ["-ClientDir", "{client}", "-OutputDir", "{work}"],
             "note": "read-only dry-run check"}
            for s in pvf_scripts
        ]}
    if client_ops:
        layers["client"] = {"ops": client_ops}

    manifest = {
        "schema": 2,
        "id": MOD_ID,
        "version": VERSION,
        "name": "hello-verify",
        "author": "DFO 115us",
        "description": ("四层示例 mod：server.boot 钩子 + 只读 PVF 校验 + 无害客户端标记文件"),
        "permissions": permissions,
        "layers": layers,
    }
    manifest_path = os.path.join(staging, "mod.json")
    with io.open(manifest_path, "w", encoding="utf-8", newline="\n") as f:
        f.write(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n")

    # ---- 4) 打包（组织文件在根；固定顺序与时间戳 → 可复现）----
    zip_path = os.path.join(args.out, "%s-%s.zip" % (MOD_ID, VERSION))
    if os.path.exists(zip_path):
        os.remove(zip_path)

    members = []
    for root, _dirs, files in os.walk(staging):
        for name in files:
            full = os.path.join(root, name)
            rel = os.path.relpath(full, staging).replace(os.sep, "/")
            members.append((rel, full))
    # mod.json 必须排第一（可读性），其余按路径排序
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
    print("layers: %s" % ", ".join(layers.keys()))
    print("perms : %s" % ", ".join(permissions))

    # ---- 5) 自证：跑一次 modkit verify ----
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
    print("  modkit plan    --client CLIENTDIR --mod \"%s\" --root LAUNCHERROOT" % zip_path)
    print("  modkit install --client CLIENTDIR --mod \"%s\" --root LAUNCHERROOT" % zip_path)
    return 0


if __name__ == "__main__":
    sys.exit(main())

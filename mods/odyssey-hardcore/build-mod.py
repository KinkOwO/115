#!/usr/bin/env python3
"""打包 odyssey.hardcore（奥德赛强化：奥德赛副本内 血量 ×10 + 怪物伤害 ×10 + 禁消耗品 + 不可复活 + 爆率 ×5）。

与 mods/examples/*/build-mod.py 同一套路：收集 server 层 → 从源码推导钩子与权限 →
生成组织文件 mod.json → 打包（固定时间戳，字节可复现）→ 自证。

用法：
  python build-mod.py                        # 输出到 <脚本目录>/dist
  python build-mod.py --modkit <modkit.exe>  # 顺便跑一次 modkit verify
  python build-mod.py --rules-only           # 只出服务端层，不带客户端插件与范围规则

client 层（完整包）**依赖**下列两个源文件，缺任一个都直接报错退出、
绝不生成一个"看起来装上了但其实没有倍率"的包：
  1) <仓库根>\\client-patchs\\difficulty\\dist\\DifficultyRules.dll
     ← 由 client-patchs\\difficulty\\build-dll.cmd 生成（先跑它）
  2) 本目录 client\\rules.d\\odyssey.hardcore.json
     ← 本 mod 自带的"只对奥德赛副本"范围规则（dungeonIds 来自真实 PVF 解析）

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

MOD_ID = "odyssey.hardcore"
VERSION = "1.0.0"
ZIP_DATE = (2026, 10, 7, 0, 0, 0)  # 固定时间戳 → 可复现
NAME = "奥德赛强化（奥德赛内：血量 ×10 + 怪物伤害 ×10 + 禁消耗品 + 死亡不可复活 + 爆率 ×5）"
DESCRIPTION = (
    "只在**奥德赛模式**的副本里生效，出本/城镇/普通副本一律不受影响："
    "① 怪物血量 ×10、② 怪物伤害 ×10 —— 由随包分发的客户端插件 DifficultyRules.dll "
    "（client 层 file.add 到 .115us-mods/）读**本 mod 自带**的范围规则 "
    ".115us-mods/rules.d/odyssey.hardcore.json（dungeonIds = 真实 PVF 里 "
    "[dungeon mode script] arad odyssey 的全部副本，percent/attackPercent = 1000）实现；"
    "插件只做内存读写，不改 DFO.exe、不落 dinput8.dll、无 inline 钩子，"
    "写前逐字节核对现场，对不上就跳过并记日志。"
    "③ 副本内禁用消耗品（可携带，城镇不受影响）、④ 死亡不可复活 —— 服务端 internal/modpolicy "
    "挂在既有业务点上（CMD44 / CMD41），不是第二套玩法表。"
    "⑤ 掉落倍率 ×5 —— 同一份策略的掉落两张表（世界掉落 / 小怪专属池），数值来自本 mod 的 "
    "server/config.json（world_drop_percent / monster_item_percent，100 = 1.00 倍）。"
    "服务端层改动需重启服务端；客户端插件倍率改动只需改 rules.d 里的 json 后重启客户端。"
    "兼容性：本 mod 与 difficulty.rules 都投 .115us-mods/DifficultyRules.dll，两个都装会被 "
    "modkit 以「目标已被 mod X 占用」阻断（有意为之：同一个引擎只允许一个实例，"
    "避免双份加载把倍率叠成 ×100）。二选一即可。"
)
HERE = os.path.dirname(os.path.abspath(__file__))
REPO_ROOT = os.path.dirname(os.path.dirname(HERE))  # <仓库根>/mods/odyssey-hardcore → <仓库根>

# client 层两个源文件
DLL_SOURCE = os.path.join(REPO_ROOT, "client-patchs", "difficulty", "dist", "DifficultyRules.dll")
RULES_SOURCE = os.path.join(HERE, "client", "rules.d", "odyssey.hardcore.json")

PLUGIN_DIR = ".115us-mods"
PLUGIN_NAME = "DifficultyRules.dll"
RULES_NAME = "odyssey.hardcore.json"


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


def make_client_ops():
    """client 层两条 file.add：插件 DLL + 本 mod 自带的"只对奥德赛副本"范围规则。

    为什么插件与规则**都在这个 mod 里**：用户只装 odyssey.hardcore 一个 mod 时，
    插件（引擎）与范围规则（只对奥德赛）必须一起到位；把范围规则交给用户的
    rules.json 去写，就等于要求用户自己记住那 56 个副本 id —— 那不是 mod 该干的事。
    """
    for path, hint in ((DLL_SOURCE, "先跑 client-patchs\\difficulty\\build-dll.cmd"),
                       (RULES_SOURCE, "本 mod 的 client\\rules.d\\odyssey.hardcore.json 缺失")):
        if not os.path.isfile(path):
            raise SystemExit("FAIL: 缺少 client 层源文件 %s（%s）" % (path, hint))
    ops, disk = [], []
    for target, src, disk_src, note in (
        (PLUGIN_DIR + "/" + PLUGIN_NAME, "client/" + PLUGIN_NAME, DLL_SOURCE,
         "客户端难度引擎（宿主插件通道：客户端唯一可自动加载的槽位由 qol.client-host 占用）。"
         "规则文件 <插件目录>\\rules.json 由插件首次运行时自己生成（enabled=false 模板），"
         "不随本包落位；本包只带 rules.d 里的范围规则。"),
        (PLUGIN_DIR + "/rules.d/" + RULES_NAME, "client/rules.d/" + RULES_NAME, RULES_SOURCE,
         "本 mod 自带的范围规则：**只对奥德赛副本**（真实 PVF 解析出的 56 个 id、"
         "percent/attackPercent = 1000）。插件按文件名升序先读 rules.d、再读玩家的 rules.json，"
         "所以这份规则在奥德赛内赢过玩家的通用规则，其它副本仍按玩家规则（或原版）。"),
    ):
        ops.append({"kind": "file.add", "target": target, "source": src,
                    "size": os.path.getsize(disk_src), "sha256": sha256_of(disk_src),
                    "note": note})
        disk.append((src, disk_src))
    ops.sort(key=lambda op: op["target"])  # 清单顺序稳定：只受内容影响，不受源码书写顺序影响
    return ops, disk


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--out", default=os.path.join(HERE, "dist"))
    ap.add_argument("--modkit", default=None)
    ap.add_argument("--rules-only", action="store_true",
                    help="只出服务端层（禁消耗品 + 不可复活 + 掉落 ×5），不含客户端插件与范围规则")
    args = ap.parse_args()

    staging = os.path.join(args.out, "staging")
    if os.path.isdir(staging):
        shutil.rmtree(staging)
    os.makedirs(staging, exist_ok=True)

    # ---- 1) server 层（Go 源码 + 内嵌 config.json）----
    server_src = os.path.join(HERE, "server")
    if not os.path.isdir(server_src):
        print("FAIL: missing server/ layer", file=sys.stderr)
        return 1
    if not os.path.isfile(os.path.join(server_src, "mod.go")):
        print("FAIL: server/mod.go missing", file=sys.stderr)
        return 1
    if not os.path.isfile(os.path.join(server_src, "config.json")):
        print("FAIL: server/config.json missing（规则开关是 mod 的一部分）", file=sys.stderr)
        return 1
    n_server = stage_tree(server_src, os.path.join(staging, "server"))
    print("server layer: %d file(s)（含 config.json）" % n_server)

    # ---- 2) client 层（插件 DLL + rules.d 范围规则）----
    client_ops = []
    if not args.rules_only:
        client_ops, client_disk = make_client_ops()
        for rel, disk_src in client_disk:
            dest = os.path.join(staging, rel.replace("/", os.sep))
            os.makedirs(os.path.dirname(dest), exist_ok=True)
            shutil.copy2(disk_src, dest)
        print("client layer: %d file(s)" % len(client_ops))
        for op in client_ops:
            print("  file.add %-46s %8d B  %s…" % (op["target"], op["size"], op["sha256"][:16]))
        # client 层的**说明文档**：只随包可看（zip 内的普通文件），**不是安装动作** ——
        # 它不进 manifest 的 ops，modkit 不会把它落位到启动器 mods 目录。
        n_doc = 0
        for name in sorted(os.listdir(os.path.join(HERE, "client"))):
            if not name.endswith(".md"):
                continue
            src = os.path.join(HERE, "client", name)
            if not os.path.isfile(src):
                continue
            dest = os.path.join(staging, "client", name)
            os.makedirs(os.path.dirname(dest), exist_ok=True)
            shutil.copy2(src, dest)
            n_doc += 1
            print("  doc(仅包内)   %-40s %8d B" % ("client/" + name, os.path.getsize(src)))
    else:
        print("client layer: 无（--rules-only）")

    # ---- 3) 从源码推导钩子与权限（不手写第二份清单）----
    code = read_text(os.path.join(server_src, "mod.go"))
    hooks = []
    if "RegisterBoot(" in code:
        hooks.append({"name": "server.boot",
                      "note": "把奥德赛规则 + 掉落倍率写进 internal/modpolicy（生效点）"})
    if "RegisterConsole(" in code:
        hooks.append({"name": "console.command",
                      "note": "一次性诊断命令：status / what"})
    if not hooks:
        print("FAIL: server/mod.go 没有登记任何钩子", file=sys.stderr)
        return 1

    permissions = sorted({"server.hook"} | ({"client.file.write"} if client_ops else set()))
    # requires 只在真的用到宿主插件通道时声明（rules-only 包不写，避免白挂一条依赖）。
    requires = ["qol.client-host"] if client_ops else []

    # ---- 4) 组织文件 ----
    manifest = {
        "schema": 2,
        "id": MOD_ID,
        "version": VERSION,
        "name": NAME,
        "author": "DFO 115us",
        "description": DESCRIPTION if not args.rules_only else (
            DESCRIPTION.replace(
                "① 怪物血量 ×10、② 怪物伤害 ×10 —— 由随包分发的客户端插件 DifficultyRules.dll "
                "（client 层 file.add 到 .115us-mods/）读**本 mod 自带**的范围规则 "
                ".115us-mods/rules.d/odyssey.hardcore.json（dungeonIds = 真实 PVF 里 "
                "[dungeon mode script] arad odyssey 的全部副本，percent/attackPercent = 1000）实现；"
                "插件只做内存读写，不改 DFO.exe、不落 dinput8.dll、无 inline 钩子，"
                "写前逐字节核对现场，对不上就跳过并记日志。",
                "① 怪物血量 ×10 / ② 怪物伤害 ×10 **不在本包内**（本变体不含客户端插件）："
                "要血量与伤害倍率请装完整包。")),
        "permissions": permissions,
        "layers": dict(
            [("server", {"package": ".", "hooks": hooks})]
            + ([("client", {"ops": client_ops})] if client_ops else [])
        ),
    }
    if requires:
        manifest["requires"] = requires
    manifest_path = os.path.join(staging, "mod.json")
    with io.open(manifest_path, "w", encoding="utf-8", newline="\n") as f:
        f.write(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n")
    # 源码指纹：用来回答"这个包到底是从哪份 mod.go 打出来的"
    print("mod.go sha256: %s" % sha256_of(os.path.join(server_src, "mod.go")))

    # ---- 5) 打包 ----
    suffix = "-rules-only" if args.rules_only else ""
    zip_path = os.path.join(args.out, "%s-%s%s.zip" % (MOD_ID, VERSION, suffix))
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
    print("variant: %s" % ("rules-only（只服务端层）" if args.rules_only else "full（服务端 + 客户端插件）"))
    print("built  : %s" % zip_path)
    print("size   : %d bytes" % os.path.getsize(zip_path))
    print("sha256 : %s" % sha256_of(zip_path))
    print("perms  : %s" % (", ".join(permissions) if permissions else "（无）"))
    print("requires: %s" % (", ".join(requires) if requires else "（无）"))
    print("hooks  : %s" % ", ".join(h["name"] for h in hooks))
    print("members:")
    for rel, full in members:
        print("  %-48s %8d B" % (rel, os.path.getsize(full)))
    if client_ops:
        print("client : %d 条 file.add（插件 + 本 mod 自带范围规则），**无 exe.patch**" % len(client_ops))
    else:
        print("client : 无（--rules-only）")

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
    print("  modkit verify  --mod \"%s\"" % zip_path)
    print("  modkit plan    --client <客户端目录> --mod \"%s\" --root <启动器根>" % zip_path)
    print("  modkit install --client <客户端目录> --mod \"%s\" --root <启动器根>" % zip_path)
    print("  # server 层是 Go 包：装完要重编服务端（mods/odyssey.hardcore 参与编译）")
    print("  # 客户端插件倍率改 rules.d 里的 json 后重启客户端即可，不用重装 mod")
    return 0


if __name__ == "__main__":
    sys.exit(main())

#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""DFO 115us 简易 GM（SQLite 与 PostgreSQL 两档通用）。

能力
  list                          账号点券 + 角色列表（id / 名字 / 等级 / 经验 / 金币）
  set --name <角色> [--level N] [--cera +N] [--gold +N] [--item 3037x10]
                                修改；点券/金币/物品走 cmd/admin 的单事务 + 幂等键 + 审计路径
  history                       本账号已记录的操作

设计约束
  * 读取走 `dfo-tool accountlist`（引擎中立的只读概览），本脚本不再直接连库：
    存档是 SQLite 文件还是 PostgreSQL 库，对 GM 命令没有区别。
  * 写操作统一交给 Go 工具（`cmd/admin` 发点券/金币/物品，`dfo-tool setlevel` 改等级），
    它们本来就按 `internal/database.Open` 选引擎，带审计与幂等键；本脚本不直接改库。
  * 改等级按 PVF 的累计经验阈值写 experience = Thresholds[level-2]（该等级起点），
    由 `dfo-tool setlevel` 实现（需要 PVF 的 progression 域）。
"""
import argparse
import json
import os
import pathlib
import socket
import subprocess
import sys
import time
from urllib.parse import urlparse

import storage_profile

# 控制台是 chcp 65001（GM.cmd 设的），显式按 UTF-8 输出，避免中文角色名变成乱码。
_reconfig_out = getattr(sys.stdout, "reconfigure", None)
if callable(_reconfig_out):
    _reconfig_out(encoding="utf-8")
_reconfig_err = getattr(sys.stderr, "reconfigure", None)
if callable(_reconfig_err):
    _reconfig_err(encoding="utf-8")

REPO = pathlib.Path(__file__).resolve().parents[1]
CONFIG = REPO / "server/work/dfo-lan/runtime/storage/local.json"
MODULE = REPO / "server/work/dfo-lan"
GO = pathlib.Path("C:/Game/dof/115us/tools/go/bin/go.exe")
ARCHIVE = REPO / "server/work/client-build/Script.inner.pvf"


def storage(args=None) -> dict:
    """读取活动存储档，并按唯一规则判定引擎（见 storage_profile）。

    引擎判定的真源是服务端的 internal/database.EngineForConfig：GM 与启动器、服务端
    必须对同一份 local.json 得出同一个答案，否则会出现「PostgreSQL 在跑、GM 却在看
    另一个空库」这类错觉（2026-10-05 业主转达的 pgsql 端无法登录）。"""
    path = pathlib.Path(getattr(args, "config", None) or CONFIG)
    if not path.is_absolute():
        path = REPO / path
    if not path.exists():
        sys.exit(f"找不到存储配置: {path}")
    cfg = json.loads(path.read_text(encoding="utf-8-sig"))
    driver = storage_profile.storage_driver(cfg)
    if driver not in ("sqlite", "postgres"):
        sys.exit(f"存储档 driver={driver} 不受支持（只认 sqlite / postgres）: {path}")
    return {"driver": driver, "raw": cfg, "config": str(path)}


def pg_endpoint(st: dict):
    """PostgreSQL 档返回 (host, port)，其它档返回 None。"""
    if st["driver"] != "postgres":
        return None
    dsn = str(st["raw"].get("postgres_dsn", "") or "").strip()
    if not dsn:
        sys.exit(f"存储档 driver=postgres 但没有 postgres_dsn: {st['config']}")
    parsed = urlparse(dsn)
    return (parsed.hostname or "127.0.0.1", parsed.port or 5432)


def require_reachable(st: dict) -> None:
    """PostgreSQL 档先确认库在监听：这是「连不上」与「账号不在这个库」的分界。"""
    endpoint = pg_endpoint(st)
    if endpoint is None:
        return
    host, port = endpoint
    try:
        with socket.create_connection((host, int(port)), timeout=0.5):
            return
    except OSError:
        sys.exit(
            f"PostgreSQL {host}:{port} 没有在监听（配置 {st['config']}）。\n"
            "  先运行 scripts\\启动服务端.cmd（或挂起服务端）再执行 GM；"
            "服务端启动日志里的 `storage: engine=postgres target=<库名>` 是它真正打开的库。"
        )


def go_env() -> dict:
    env = dict(os.environ)
    env["GOPROXY"] = "https://goproxy.cn,direct"
    env["GOSUMDB"] = "off"
    env["GOTOOLCHAIN"] = "local"
    env["CGO_ENABLED"] = "0"
    env.setdefault("DFO_PVF_ARCHIVE", "../client-build/Script.inner.pvf")
    return env


def run_admin(st: dict, extra: list) -> int:
    cmd = [str(GO), "run", "./cmd/admin", "-storage", st["config"],
           "-pvf-archive", str(ARCHIVE)] + extra
    print("→ cmd/admin " + " ".join(extra), flush=True)
    print("  （首次会准备 PVF 目录，约 30~60 秒）", flush=True)
    return subprocess.run(cmd, cwd=MODULE, env=go_env()).returncode


def run_setlevel(st: dict, extra: list) -> int:
    cmd = [str(GO), "run", "./cmd/dfo-tool", "setlevel", "-config", st["config"],
           "-pvf-archive", str(ARCHIVE)] + extra
    print("→ dfo-tool setlevel " + " ".join(extra), flush=True)
    print("  （只准备 progression 一个域，通常几秒）", flush=True)
    return subprocess.run(cmd, cwd=MODULE, env=go_env()).returncode


def summarize(st: dict, account: str = "") -> dict:
    """调用引擎中立的 accountlist，拿回账号/角色概览（只读）。"""
    cmd = [str(GO), "run", "./cmd/dfo-tool", "accountlist",
           "-config", st["config"], "-json"]
    if account:
        cmd += ["-account", account]
    result = subprocess.run(cmd, cwd=MODULE, env=go_env(), stdout=subprocess.PIPE,
                            text=True, encoding="utf-8", errors="replace")
    if result.returncode != 0:
        sys.exit(f"accountlist 读取存储失败（退出码 {result.returncode}；配置 {st['config']}）")
    line = result.stdout.strip().splitlines()[-1] if result.stdout.strip() else ""
    try:
        return json.loads(line)
    except ValueError:
        sys.exit(f"accountlist 输出无法解析: {line[:200]}")


def pick_account(summary: dict, wanted: str = "") -> dict:
    accounts = summary.get("accounts") or []
    if wanted:
        for account in accounts:
            if account.get("username") == wanted:
                return account
        sys.exit(f"没有账号 {wanted}；实得: {', '.join(a['username'] for a in accounts) or '(无)'}")
    if len(accounts) == 1:
        return accounts[0]
    if not accounts:
        sys.exit("存储里没有任何账号")
    sys.exit("存储里有多个账号，请用 --account 指定：" + ", ".join(a["username"] for a in accounts))


def find_character(account: dict, name: str) -> dict:
    for role in account.get("characters") or []:
        if str(role.get("name", "")).lower() == name.lower():
            return role
    sys.exit(f"账号 {account['username']} 里没有角色 {name}；可用: "
             + ", ".join(str(r.get("name")) for r in account.get("characters") or []))


def gold_text(role: dict) -> str:
    return "-" if role.get("gold") is None else str(role["gold"])


def cmd_list(args) -> int:
    st = storage(args)
    require_reachable(st)
    summary = summarize(st, args.account or "")
    print(f"存储: {summary.get('driver')}  {summary.get('target')}")
    for account in summary.get("accounts") or []:
        print(f"\n账号 {account.get('username')} (id={account.get('id')})   点券: {account.get('cera')}")
        if not account.get("characters"):
            print("  （没有角色）")
            continue
        print("  id   名字            等级   经验                金币")
        for role in account["characters"]:
            print(f"  {role['id']:<4} {str(role['name']):<14} {str(role['level']):<6} "
                  f"{str(role['experience']):<19} {gold_text(role)}")
    return 0


def parse_delta(text: str) -> int:
    t = text.strip()
    if t.startswith("+"):
        return int(t[1:])
    return int(t)


def cmd_set(args) -> int:
    st = storage(args)
    require_reachable(st)
    before = summarize(st, args.account or "")
    account = pick_account(before, args.account or "")
    target = find_character(account, args.name)

    if args.level is None and args.cera is None and args.gold is None and not args.item:
        sys.exit("没有要修改的内容：至少给一个 --level / --cera / --gold / --item")

    # 幂等键带 pid：同一秒内连开两次 GM 也不会互相吞掉发放。
    stamp = f"{int(time.time())}-{os.getpid()}"
    if args.level is not None:
        level_args = [
            "-account", account["username"],
            "-name", target["name"],
            "-level", str(args.level),
            "-reason", args.reason,
            "-operator", args.operator,
        ]
        if not args.preview:
            level_args += ["-apply", "-grant-id", f"gm-level-{stamp}"]
        if run_setlevel(st, level_args):
            return 1

    if args.cera is not None or args.gold is not None or args.item:
        extra = [
            "-account", account["username"],
            "-character", str(target["id"]),
            "-grant-id", f"gm-grant-{stamp}",
            "-reason", args.reason,
            "-operator", args.operator,
        ]
        if args.cera is not None:
            extra += ["-cera", str(parse_delta(args.cera))]
        if args.gold is not None:
            extra += ["-gold", str(parse_delta(args.gold))]
        if args.item:
            extra += ["-item", args.item]
        if run_admin(st, extra):
            return 1

    after = summarize(st, args.account or "")
    print("\n改前:", target)
    print("改后:", find_character(pick_account(after, account["username"]), args.name))
    return 0


def cmd_history(args) -> int:
    st = storage(args)
    require_reachable(st)
    return run_admin(st, ["-account", args.account, "-history"])


def cmd_help(args) -> int:
    """中文用法横幅。

    放在这里而不是 GM.cmd 里：.cmd 会被 cmd.exe 按控制台代码页解码，中文写进 echo 行
    会在某些控制台上被拆成乱码命令（AGENTS.md §0.4.2）——.cmd 因此保持纯 ASCII，
    中文一律由 Python 打印（本脚本显式按 UTF-8 输出）。"""
    print("=" * 60)
    print(" DFO 115us 简易 GM（SQLite / PostgreSQL 两档通用）")
    print("-" * 60)
    print("  查看：      scripts\\GM.cmd list")
    print("  等级：      scripts\\GM.cmd set --name 角色名 --level 50        （加 --preview 只预览）")
    print("  点券/金币： scripts\\GM.cmd set --name 角色名 --cera +10000 --gold +5000000")
    print("  发物品：    scripts\\GM.cmd set --name 角色名 --item 3037x10,20002")
    print("  操作记录：  scripts\\GM.cmd history")
    print("-" * 60)
    print("  * 引擎按 runtime\\storage\\local.json 的 driver 自动判定（与服务端同一条规则）：")
    print("    SQLite 档直接读库文件；PostgreSQL 档需要库在监听，否则会提示先启动服务端。")
    print("  * 读取走 dfo-tool accountlist（只读、引擎中立）；写操作走 cmd/admin 与")
    print("    dfo-tool setlevel 的单事务 + 幂等键 + 审计路径；重复执行同一个 grant-id 只发一次。")
    print("  * 首次执行会准备 PVF 目录，约 30~60 秒（改等级只准备 progression 一个域，几秒）。")
    print("  * 角色在线时背包/余额仍用内存里的旧值，重选角色即可看到新值。")
    print("  * 改等级按 PVF 的累计经验阈值写 experience = Thresholds[level-2]（该等级起点），")
    print("    同步保证不会触发 level exceeds cumulative experience；不改技能点。")
    print("=" * 60)
    return 0


def main() -> int:
    p = argparse.ArgumentParser(prog="GM", description="DFO 115us 简易 GM（SQLite / PostgreSQL 通用）")
    sub = p.add_subparsers(dest="cmd", required=True)
    common = argparse.ArgumentParser(add_help=False)
    common.add_argument("--config", default=None, help="存储档路径（默认 runtime/storage/local.json）")
    listing = sub.add_parser("list", parents=[common], help="账号点券 + 角色列表")
    listing.add_argument("--account", default=None, help="只列该账号（默认：全部）")
    listing.set_defaults(func=cmd_list)
    sub.add_parser("help", help="打印中文用法横幅").set_defaults(func=cmd_help)

    s = sub.add_parser("set", parents=[common], help="修改等级 / 点券 / 金币 / 物品")
    s.add_argument("--name", required=True, help="角色名")
    s.add_argument("--level", type=int, default=None, help="目标等级（按 PVF 累计经验阈值写）")
    s.add_argument("--cera", default=None, help="点券增减，如 +10000 或 -500")
    s.add_argument("--gold", default=None, help="金币增减，如 +5000000")
    s.add_argument("--item", default=None, help="物品，如 3037x10,20002")
    s.add_argument("--preview", action="store_true", help="改等级时只预览、不写库")
    s.add_argument("--account", default=None, help="账号名（默认：存储里唯一的账号）")
    s.add_argument("--reason", default="gm command line", help="审计原因")
    s.add_argument("--operator", default="local-operator", help="操作者（写审计）")
    s.set_defaults(func=cmd_set)

    h = sub.add_parser("history", parents=[common], help="本账号操作记录")
    h.add_argument("--account", default="probe")
    h.set_defaults(func=cmd_history)

    args = p.parse_args()
    return args.func(args)


if __name__ == "__main__":
    sys.exit(main())

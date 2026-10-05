"""Portable handoff launcher for EXISTING locally configured storage."""

import argparse
import datetime
import json
import os
import pathlib
import socket
import subprocess
import sys
import time
from urllib.parse import urlparse
from ensure_inner_pvf import ensure as ensure_inner_pvf
from prepare_inner_pvf import prepare as prepare_inner_pvf
from repair_profile import load_profile

PROJECT = pathlib.Path(__file__).resolve().parent.parent
ROOT = PROJECT.parent.parent
STORAGE = PROJECT / "runtime/storage"
FLAGS = getattr(subprocess, "CREATE_NO_WINDOW", 0)
DEFAULT_PVF_PROFILE = PROJECT / "configs/pvf-default.json"
# 内层 PVF（PVF 直读模式的输入）的落地位置。
# ⚠️ 注意 ROOT 在本文件里是 `server/`（= PROJECT.parent.parent），不是整合包根；
# client-build 与 dfo-lan 同级，都在 server/work 下，所以是 PROJECT.parent 而非 ROOT。
# `configs/pvf-default.json` 的 `DFO_PVF_ARCHIVE: ../client-build/Script.inner.pvf` 也是
# 以 dfo-lan 为基准（PROJECT）解析的，这里必须与其对齐。
INNER_PVF = PROJECT.parent / "client-build/Script.inner.pvf"
INNER_MANIFEST = PROJECT.parent / "client-build/Script.inner.manifest.json"


def resolved(value):
 p = pathlib.Path(value).expanduser()
 return (p if p.is_absolute() else ROOT / p).resolve()


def listening(host, port):
 try:
  with socket.create_connection((host, port), timeout=0.5):
   return True
 except OSError:
  return False


def storage_driver(cfg):
 """Which engine this local.json selects.

 Mirror of the server's internal/database.engineForConfig - the launcher and the server
 read the same file and must never disagree, or PostgreSQL gets started while the server
 opens a leftover SQLite file (2026-10-05: pgsql 端无法登录). An explicit driver wins, a
 named DSN means PostgreSQL, and only a config that names nothing but sqlite_path is
 SQLite (the shape the 20261004 upgrade package's migration tool writes)."""
 driver = str(cfg.get("driver", "") or "").strip().lower()
 if driver:
  return driver
 if str(cfg.get("postgres_dsn", "") or "").strip():
  return "postgres"
 if str(cfg.get("sqlite_path", "") or "").strip():
  return "sqlite"
 return "postgres"


def configuration():
 settings = ROOT / "launcher.local.json"
 if not settings.exists():
  raise RuntimeError(
   "Copy launcher.example.json to launcher.local.json and set client_dir."
  )
 local = json.loads(settings.read_text(encoding="utf-8-sig"))
 storage = STORAGE / "local.json"
 if not storage.exists():
  raise RuntimeError(
   "Storage missing. Follow README first-time setup; no database was changed."
  )
 cfg = json.loads(storage.read_text(encoding="utf-8-sig"))
 driver = storage_driver(cfg)
 if driver == "sqlite":
  # No server to start: the engine opens (and creates) the database file itself.
  # pg stays None so every PostgreSQL-specific step is skipped, not attempted.
  path = str(cfg.get("sqlite_path", "") or "")
  if not path:
   raise RuntimeError(
    "Storage driver 'sqlite' requires sqlite_path in runtime/storage/local.json."
   )
  return local, cfg, None
 if driver != "postgres":
  raise RuntimeError(f"Unsupported storage driver '{driver}'.")
 dsn = str(cfg.get("postgres_dsn", "") or "").strip()
 if not dsn:
  # Same message the server gives, so both ends of the same config fail the same way
  # instead of one of them dying with a bare KeyError.
  raise RuntimeError(
   "Storage driver 'postgres' requires postgres_dsn in runtime/storage/local.json."
  )
 pg = urlparse(dsn)
 if pg.hostname != "127.0.0.1":
  raise RuntimeError("This development profile requires local loopback storage.")
 return local, cfg, pg


def start_storage(cfg, pg):
 if pg is None:
  # SQLite profile: nothing to start, and nothing to wait for.
  return
 if not listening(pg.hostname, pg.port):
  data = pathlib.Path(cfg.get("postgres_data", "")).resolve()
  if data != (STORAGE / "pgdata").resolve():
   raise RuntimeError(
    "Database offline; external data directories must be started by their owner."
   )
  pgctl = pathlib.Path(cfg["postgres_bin"]) / "pg_ctl.exe"
  if not (data / "PG_VERSION").exists() or not pgctl.exists():
   raise RuntimeError("Existing PostgreSQL data or pg_ctl missing.")
  with (STORAGE / "launcher-postgres.log").open("ab") as log:
   r = subprocess.run(
    [
     str(pgctl),
     "-D",
     str(data),
     "-l",
     str(STORAGE / "postgres.log"),
     "-w",
     "-t",
     "30",
     "start",
    ],
    stdout=log,
    stderr=log,
    creationflags=FLAGS,
    timeout=40,
   )
  if r.returncode or not listening(pg.hostname, pg.port):
   raise RuntimeError("PostgreSQL did not start; inspect launcher-postgres.log.")


def gateway_configuration(args, local):
 binary = resolved(
  "work/dfo-lan/bin/wireprobe-handoff-source.exe"
  if args.source_build else local["server_binary"]
 )
 profile = args.repair_profile
 if not profile and not args.json_mode and not args.client_only and not args.storage_only:
  profile = DEFAULT_PVF_PROFILE
 required, environment = [], {}
 if profile:
  profile_binary, required, environment = load_profile(profile, PROJECT)
  if args.source_build and not args.repair_profile:
   required = [binary if p == profile_binary else p for p in required]
  else:
   # profile 的 binary 是权威：默认档就是官方 `启动游戏.cmd` / `启动服务端.cmd`
   # 走的那条路径（两者都直接调本脚本、不带任何 binary 参数），必须与官方一致。
   binary = profile_binary
 return binary, required, environment


def _ensure_inner_pvf(client, args):
 """按需生成/刷新内层 PVF（PVF 直读模式的输入数据）。

 与启动器侧 internal/pvfprep 的四态门禁同语义：无 → 生成；有但无清单 → 重建
 （不可信）；有且清单与客户端三件套一致 → 复用；不一致 → 重建。

 为什么放在服务端侧：玩家可能不经启动器、直接用 `启动服务端.cmd` 起服，那条路径
 不会经过启动器的更新检查点。放在这里保证"任何起服方式都能自愈"。

 `--json-mode` 不走直读，由调用点按 profile_env 判定后跳过，这里不再自判。
 """
 # --client-only 只起客户端、连别的机器上的服务端，本机不需要内层归档。
 if args.client_only:
  return
 try:
  result = ensure_inner_pvf(
   client,
   INNER_PVF,
   INNER_MANIFEST,
   prepare_inner_pvf,
   log=print,
  )
 except Exception as exc:
  # 不阻断：内层归档只影响直读模式，玩家仍可显式 --json-mode 回退；
  # 且这里的失败多为"客户端没配好"这类可恢复情形，如实报错更有用。
  print("WARNING: 内层 PVF 未就绪：%s" % exc)
  return
 if result["generated"]:
  print("内层 PVF 已生成（耗时 %.1fs）" % result["elapsed"])
 else:
  print("内层 PVF 无需重建：%s" % result["reason"])


def launch_environment(args, profile_env):
 env = os.environ.copy()
 if args.json_mode:
  for key in list(env):
   if key.startswith("DFO_PVF_"):
    del env[key]
 if profile_env:
  # Native profiles preserve scenario/Odyssey and other gameplay switches.
  if "DFO_PVF_CATALOGS" not in profile_env:
   env.pop("DFO_SKILL_RELEASE", None)
   env.pop("DFO_ODYSSEY_REWARDS_PILOT", None)
  env.update(profile_env)
 return env


def main():
 parser = argparse.ArgumentParser()
 parser.add_argument(
  "--check", action="store_true", help="Read-only dependency check; starts nothing"
 )
 parser.add_argument("--storage-only", action="store_true")
 sources = parser.add_mutually_exclusive_group()
 sources.add_argument("--repair-profile", help="Override the default PVF profile; paths relative to dfo-lan")
 sources.add_argument("--json-mode", action="store_true", help="Explicit legacy JSON mode using launcher.local.json server_binary")
 parser.add_argument(
  "--server-only",
  action="store_true",
  help="Run storage and game gateway without client",
 )
 parser.add_argument(
  "--source-build",
  action="store_true",
  help="Use bin/wireprobe-handoff-source.exe with the selected data mode",
 )
 parser.add_argument(
  "--client-only",
  action="store_true",
  help="Run the client against a server on another machine (DFO_LAN_HOST); starts no local server or storage",
 )
 args = parser.parse_args()
 local, cfg, pg = configuration()
 channel_identity = local.get("channel_identity", False)
 if type(channel_identity) is not bool:
  raise ValueError("channel_identity must be a JSON boolean")
 client = resolved(local["client_dir"])
 binary, profile_required, profile_env = gateway_configuration(args, local)
 helper = PROJECT.parent / "dfo_probe_tools/channel_probe.py"
 required = (
  [helper, helper.parent / "probe.exe", client / "DFO.exe", client / "Script.pvf", client / "sk.dat"]
  if args.client_only
  else (
   [helper, binary]
   if args.server_only
   else [
    helper,
    helper.parent / "probe.exe",
    binary,
    client / "DFO.exe",
    client / "Script.pvf",
    client / "sk.dat",
   ]
  )
 )
 required.append(helper.parent / "catalog_startup.py")
 for path in required:
  if not path.is_file():
   raise RuntimeError("Missing dependency: " + str(path))
 # PVF 直读模式的内层归档是按需生成的本地产物（不随包发布、上游也没有自动生成入口）。
 # 这里在"校验 profile 依赖"之前自愈：默认 profile 就把它列进 required，
 # 不先补齐的话下一步就是 `open inner PVF` 失败。
 if profile_env.get("DFO_PVF_CATALOGS"):
  _ensure_inner_pvf(client, args)
 for path in profile_required:
  if not path.is_file():
   raise RuntimeError("Missing repair profile dependency: " + str(path))
 if args.check:
  print(
   "Paths OK. Storage:",
   f"SQLite {cfg.get('sqlite_path')}"
   if pg is None
   else f"PostgreSQL: {listening(pg.hostname, pg.port)}",
  )
  print("Binary:", binary)
  print("Data mode:", "PVF direct" if profile_env.get("DFO_PVF_CATALOGS") else "JSON / explicit profile")
  print("Client:", client)
  return
 if os.name != "nt":
  raise RuntimeError("The game launcher requires Windows x64.")
 # Admin check: probe.exe degrades gracefully without admin; keep note for reference.
 # if not ctypes.windll.shell32.IsUserAnAdmin():raise RuntimeError('Run Start-DFO.cmd as administrator.')
 if not args.storage_only and not args.client_only and listening("127.0.0.1", 7001):
  raise RuntimeError("Port7001 in use; inspect existing session before retrying.")
 # 客户端模式连接别的机器上的服务端，本机不启动存储。
 if not args.client_only:
  start_storage(cfg, pg)
 if args.storage_only:
  print("Existing storage ready.")
  return
 tag = (
  "roles_persist_select_actor_town_world_live_detail_dungeon_manual_"
  + datetime.datetime.now().strftime("%Y%m%d_%H%M%S_%f")
  + "_next37"
 )
 out = PROJECT / "runtime" / tag
 out.mkdir(parents=True)
 env = launch_environment(args, profile_env)
 env["DFO_CLIENT_DIR"] = str(client)
 env["DFO_SERVER_BINARY"] = str(binary)
 env["DFO_CHANNEL_IDENTITY"] = "1" if channel_identity else "0"
 env["DFO_ENABLE_OBSERVER"] = "0"
 mode = "server-only" if args.server_only else ("client-only" if args.client_only else "interactive")
 with (
  (out / "helper.out").open("wb") as stdout,
  (out / "helper.err").open("wb") as stderr,
 ):
  process = subprocess.Popen(
   [sys.executable, str(helper), tag, mode],
   cwd=ROOT,
   env=env,
   stdin=subprocess.DEVNULL,
   stdout=stdout,
   stderr=stderr,
   creationflags=FLAGS,
  )
 # The helper already permits 180 seconds for PVF source preparation.
 # Wait longer here so the outer launcher cannot time out first.
 startup_checks = 2100 if profile_env.get("DFO_PVF_CATALOGS") else 300
 for _ in range(startup_checks):
  if (out / "run.json").exists():
   if args.server_only:
    run_info = json.loads((out / "run.json").read_text())
    print(
     f"Game server started successfully (PID: {run_info.get('server_pid')}). Port 7001 is active."
    )
   else:
    print("Client launch requested. UI/gameplay acceptance remains separate.")
   print("Logs:", out)
   return
  if process.poll() is not None:
   raise RuntimeError("Startup stopped; inspect " + str(out / "helper.err"))
  time.sleep(0.1)
 raise RuntimeError("Startup timeout; inspect logs before retry: " + str(out))


if __name__ == "__main__":
 try:
  main()
 except Exception as exc:
  print("ERROR:", exc, file=sys.stderr)
  sys.exit(1)

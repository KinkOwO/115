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
from repair_profile import load_profile

PROJECT = pathlib.Path(__file__).resolve().parent.parent
ROOT = PROJECT.parent.parent
STORAGE = PROJECT / "runtime/storage"
FLAGS = getattr(subprocess, "CREATE_NO_WINDOW", 0)


def resolved(value):
 p = pathlib.Path(value).expanduser()
 return (p if p.is_absolute() else ROOT / p).resolve()


def listening(host, port):
 try:
  with socket.create_connection((host, port), timeout=0.5):
   return True
 except OSError:
  return False


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
 pg = urlparse(cfg["postgres_dsn"])
 rh, rp = cfg["redis_address"].rsplit(":", 1)
 if pg.hostname != "127.0.0.1" or rh != "127.0.0.1":
  raise RuntimeError("This development profile requires local loopback storage.")
 return local, cfg, pg, rh, int(rp)


def start_storage(cfg, pg, rh, rp):
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
 if not listening(rh, rp):
  redis = pathlib.Path(cfg["redis_bin"]) / "redis-server.exe"
  if not redis.exists() or not (STORAGE / "redis.conf").exists():
   raise RuntimeError("Redis binary/config missing.")
  with (STORAGE / "redis.log").open("ab") as log:
   child = subprocess.Popen(
    [str(redis), "redis.conf"], cwd=STORAGE, stdout=log, stderr=log, creationflags=FLAGS
   )
  for _ in range(100):
   if listening(rh, rp):
    return
   if child.poll() is not None:
    break
   time.sleep(0.1)
  raise RuntimeError("Redis did not start; inspect redis.log.")


def main():
 parser = argparse.ArgumentParser()
 parser.add_argument(
  "--check", action="store_true", help="Read-only dependency check; starts nothing"
 )
 parser.add_argument("--storage-only", action="store_true")
 parser.add_argument("--repair-profile", help="Explicit repair JSON; paths relative to dfo-lan")
 parser.add_argument(
  "--server-only",
  action="store_true",
  help="Run storage and game gateway without client",
 )
 parser.add_argument(
  "--source-build",
  action="store_true",
  help="Use restored source build instead of archived39",
 )
 parser.add_argument(
  "--client-only",
  action="store_true",
  help="Run the client against a server on another machine (DFO_LAN_HOST); starts no local server or storage",
 )
 args = parser.parse_args()
 local, cfg, pg, rh, rp = configuration()
 client = resolved(local["client_dir"])
 binary = resolved(
  "work/dfo-lan/bin/wireprobe-handoff-source.exe"
  if args.source_build
  else local["server_binary"]
 )
 helper = PROJECT.parent / "dfo_probe_tools/channel_probe.py"
 profile_required, profile_env = [], {}
 if args.repair_profile:
  binary, profile_required, profile_env = load_profile(args.repair_profile, PROJECT)
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
 for path in required:
  if not path.is_file():
   raise RuntimeError("Missing dependency: " + str(path))
 for path in profile_required:
  if not path.is_file():
   raise RuntimeError("Missing repair profile dependency: " + str(path))
 if args.check:
  print(
   "Paths OK. PostgreSQL:", listening(pg.hostname, pg.port), "Redis:", listening(rh, rp)
  )
  print("Binary:", binary)
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
  start_storage(cfg, pg, rh, rp)
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
 env = os.environ.copy()
 if args.repair_profile:
  env.pop("DFO_SKILL_RELEASE", None)
  env.pop("DFO_ODYSSEY_REWARDS_PILOT", None)
  env.update(profile_env)
 env["DFO_CLIENT_DIR"] = str(client)
 env["DFO_SERVER_BINARY"] = str(binary)
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
 for _ in range(300):
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

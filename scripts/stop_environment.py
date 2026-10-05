"""Stop all local DFO environment services safely and cleanly."""

import json
import pathlib
import socket
import subprocess
import sys

import storage_profile

reconfig_out = getattr(sys.stdout, "reconfigure", None)
if callable(reconfig_out):
    reconfig_out(encoding="utf-8")
reconfig_err = getattr(sys.stderr, "reconfigure", None)
if callable(reconfig_err):
    reconfig_err(encoding="utf-8")

ROOT = pathlib.Path(__file__).resolve().parent.parent  # scripts/ 的上一级 = 仓库根
STORAGE = ROOT / "server/work/dfo-lan/runtime/storage"
FLAGS = getattr(subprocess, "CREATE_NO_WINDOW", 0)


def listening(host, port):
    try:
        with socket.create_connection((host, int(port)), timeout=0.5):
            return True
    except OSError:
        return False


def kill_by_image(name):
    try:
        subprocess.run(
            ["taskkill", "/F", "/IM", name],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            creationflags=FLAGS,
        )
    except Exception:
        pass


def stop_wireprobe():
    targets = [
        "wireprobe-dungeon39.exe",
        "wireprobe-handoff-source.exe",
        "wireprobe-pvf.exe",
        "wireprobe-channel-identity-candidate.exe",
        "wireprobe.exe",
        "wireprobe-character.exe",
        "probe.exe",
        "probe-rebuilt.exe",
    ]
    for target in targets:
        kill_by_image(target)


def load_storage_config():
    local_cfg = STORAGE / "local.json"
    if not local_cfg.exists():
        return {}
    try:
        return json.loads(local_cfg.read_text(encoding="utf-8-sig"))
    except Exception as exc:
        print(f"Warning: failed to read storage config: {exc}", file=sys.stderr)
        return {}


def storage_driver(cfg):
    """Which engine this local.json selects.

    Kept as a thin alias so existing callers and tests read naturally; the rule itself is
    in scripts/storage_profile.py, the Python mirror of the server's
    internal/database.engineForConfig. Reading it as "no driver means PostgreSQL" would make
    this script kill a PostgreSQL that a SQLite profile never used (2026-10-05: pgsql 端无法登录)."""
    return storage_profile.storage_driver(cfg)


def stop_postgres(cfg):
    # A sqlite profile has no server to stop: the database is a file the engine
    # opens and closes itself. Skipping keeps this script usable for both drivers.
    driver = storage_driver(cfg)
    if driver == "sqlite":
        print("Storage: sqlite profile, no PostgreSQL service to stop.")
        return
    if driver != "postgres":
        print(f"Storage: unsupported driver '{driver}', no PostgreSQL service to stop.", file=sys.stderr)
        return
    pg_bin = cfg.get("postgres_bin")
    pg_data = cfg.get("postgres_data")
    if pg_bin and pg_data:
        pg_ctl = pathlib.Path(pg_bin) / "pg_ctl.exe"
        data_path = pathlib.Path(pg_data)
        if pg_ctl.exists() and data_path.exists():
            print("Stopping PostgreSQL (fast checkpoint)...")
            try:
                subprocess.run(
                    [
                        str(pg_ctl),
                        "stop",
                        "-D",
                        str(data_path),
                        "-m",
                        "fast",
                        "-w",
                        "-t",
                        "15",
                    ],
                    stdout=subprocess.DEVNULL,
                    stderr=subprocess.DEVNULL,
                    creationflags=FLAGS,
                    timeout=20,
                )
            except Exception as exc:
                print(f"pg_ctl stop notice: {exc}")

    # Check port and force kill if still running
    if listening("127.0.0.1", 25438):
        print("PostgreSQL port 25438 still open, force terminating postgres.exe...")
        kill_by_image("postgres.exe")


def main():
    print("=== Stopping DFO 115us Environment ===")
    cfg = load_storage_config()

    print("Stopping game server and probe processes...")
    stop_wireprobe()

    stop_postgres(cfg)

    # Double check port states
    pg_ok = not listening("127.0.0.1", 25438)
    port7001_ok = not listening("127.0.0.1", 7001)

    print("Status summary:")
    print(f"  PostgreSQL (25438): {'Stopped' if pg_ok else 'ACTIVE (warning)'}")
    print(f"  Gateway    (7001):  {'Stopped' if port7001_ok else 'ACTIVE (warning)'}")

    if pg_ok and port7001_ok:
        print("Environment fully stopped.")
    else:
        print("Notice: some ports are still active.")


if __name__ == "__main__":
    main()

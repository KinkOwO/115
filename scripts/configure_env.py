"""DFO 115us 本地环境配置脚本
自动从当前工作目录 (cwd) 提取服务端及便携环境依赖路径，
并通过文件选择对话框引导用户选择游戏客户端目录 (DFO.exe)。
同步更新:
  1. server/launcher.local.json
  2. server/work/dfo-lan/runtime/storage/local.json
"""

import argparse
import json
import os
import pathlib
import subprocess
import sys

import storage_profile

reconfig_out = getattr(sys.stdout, "reconfigure", None)
if callable(reconfig_out):
    reconfig_out(encoding="utf-8")
reconfig_err = getattr(sys.stderr, "reconfigure", None)
if callable(reconfig_err):
    reconfig_err(encoding="utf-8")


def get_root_dir() -> pathlib.Path:
    """获取项目根目录，优先以 cwd 为准"""

    def looks_like_root(path: pathlib.Path) -> bool:
        # tools/ 已于 2026-10-04 移出仓库，因此根标志改为 server/ 加
        # 「服务端源码目录 server/work/dfo-lan 或 tools/ 任一」。
        return (path / "server").is_dir() and (
            (path / "server/work/dfo-lan").is_dir() or (path / "tools").is_dir()
        )

    cwd = pathlib.Path.cwd().resolve()
    # 如果 cwd 就是仓库根，直接采用
    if looks_like_root(cwd):
        return cwd
    # 如果 cwd 是 server 目录
    if cwd.name == "server" and looks_like_root(cwd.parent):
        return cwd.parent
    # 递归向上寻找
    for parent in cwd.parents:
        if looks_like_root(parent):
            return parent
    # 脚本所在目录回退：本脚本位于 scripts/ 之下，逐级向上找仓库根
    for base in pathlib.Path(__file__).resolve().parents:
        if looks_like_root(base):
            return base
    return cwd


def pop_file_dialog(initial_dir: str | None = None) -> str:
    """弹出文件选择对话框让用户选择游戏文件 (DFO.exe)"""
    print("\n[交互] 正在弹出文件选择对话框，请选择客户端可执行文件 (如 DFO.exe)...")

    # 1. 尝试使用 tkinter
    try:
        import tkinter as tk
        from tkinter import filedialog

        root = tk.Tk()
        root.withdraw()
        # 置顶窗口避免对话框被控制台或编辑器窗口遮挡
        root.wm_attributes("-topmost", True)
        root.update()

        init_dir = None
        if initial_dir and os.path.exists(initial_dir):
            init_dir = initial_dir

        selected = filedialog.askopenfilename(
            parent=root,
            title="请选择 DFO 游戏客户端文件 (例如 DFO.exe)",
            initialdir=init_dir,
            filetypes=[
                ("DFO 客户端文件 (DFO.exe)", "DFO.exe"),
                ("可执行文件 (*.exe)", "*.exe"),
                ("所有文件 (*.*)", "*.*"),
            ],
        )
        root.destroy()
        if selected:
            return selected
    except Exception as exc:
        print(f"[提示] Tkinter 对话框调起失败 ({exc})，尝试使用系统备用对话框...")

    # 2. Windows 备用方案：PowerShell OpenFileDialog
    if sys.platform == "win32":
        try:
            ps_code = (
                '[System.Reflection.Assembly]::LoadWithPartialName("System.Windows.Forms") | Out-Null; '
                "$d = New-Object System.Windows.Forms.OpenFileDialog; "
                '$d.Title = "请选择 DFO 游戏客户端文件 (例如 DFO.exe)"; '
                '$d.Filter = "DFO 可执行文件 (DFO.exe)|DFO.exe|所有可执行文件 (*.exe)|*.exe|所有文件 (*.*)|*.*"; '
                "if ($d.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) { Write-Output $d.FileName }"
            )
            res = subprocess.run(
                ["powershell", "-NoProfile", "-NonInteractive", "-Command", ps_code],
                capture_output=True,
                text=True,
                creationflags=getattr(subprocess, "CREATE_NO_WINDOW", 0),
            )
            if res.returncode == 0 and res.stdout.strip():
                return res.stdout.strip()
        except Exception as exc:
            print(f"[提示] PowerShell 对话框调起失败: {exc}")

    return ""


def main():
    parser = argparse.ArgumentParser(description="DFO 115us 本地环境路径配置工具")
    parser.add_argument(
        "--client-dir",
        "-c",
        type=str,
        default="",
        help="指定游戏客户端路径 (若指定则跳过图形选择对话框)",
    )
    parser.add_argument(
        "--server-binary",
        "-s",
        type=str,
        default="",
        help="指定服务端二进制相对或绝对路径 (默认保留已有或默认值)",
    )
    parser.add_argument(
        "--non-interactive",
        action="store_true",
        help="非交互模式 (不弹窗，未指定客户端路径时保留现有配置)",
    )
    args = parser.parse_args()

    root_dir = get_root_dir()
    print("=" * 60)
    print(" DFO 115us 环境路径配置向导")
    print(f" 服务端根目录 (从 cwd 解析): {root_dir}")
    print("=" * 60)

    # 目标配置文件路径
    launcher_file = root_dir / "server/launcher.local.json"
    launcher_example = root_dir / "server/launcher.example.json"
    storage_file = root_dir / "server/work/dfo-lan/runtime/storage/local.json"
    storage_example = (
        root_dir / "server/work/dfo-lan/runtime/storage/local.example.json"
    )

    # 读取现有 launcher 配置
    launcher_cfg = {}
    if launcher_file.exists():
        try:
            launcher_cfg = json.loads(launcher_file.read_text(encoding="utf-8-sig"))
            print(f"[加载] 现有 launcher 配置: {launcher_file}")
        except Exception as exc:
            print(f"[警告] 读取现有 {launcher_file} 失败: {exc}")
    elif launcher_example.exists():
        try:
            launcher_cfg = json.loads(launcher_example.read_text(encoding="utf-8-sig"))
            print(f"[加载] 从模板初始化: {launcher_example}")
        except Exception:
            pass

    existing_client_dir = launcher_cfg.get("client_dir", "")
    chosen_path_str = args.client_dir.strip()

    # 如果未通过 CLI 传入游戏路径
    if not chosen_path_str:
        if args.non_interactive:
            if existing_client_dir:
                print(f"[非交互模式] 保留现有游戏客户端路径: {existing_client_dir}")
                chosen_path_str = existing_client_dir
            else:
                print(
                    "[错误] 非交互模式且现有配置中无 client_dir，请使用 --client-dir 参数指定。"
                )
                sys.exit(1)
        else:
            selected = pop_file_dialog(initial_dir=existing_client_dir)
            if selected:
                chosen_path_str = selected
            else:
                # 用户在对话框中点击了取消或关闭
                print("\n[提示] 对话框未选择任何文件。")
                if existing_client_dir:
                    print(f"当前配置中的客户端路径为: {existing_client_dir}")
                    try:
                        resp = input(
                            f"是否保留当前路径 [{existing_client_dir}]? (Y/n，或直接输入新路径): "
                        ).strip()
                        if not resp or resp.lower() in ("y", "yes"):
                            chosen_path_str = existing_client_dir
                        elif resp.lower() in ("n", "no"):
                            print("已取消更新游戏路径。")
                            sys.exit(0)
                        else:
                            chosen_path_str = resp
                    except (EOFError, KeyboardInterrupt):
                        chosen_path_str = existing_client_dir
                else:
                    try:
                        resp = input(
                            "请输入游戏客户端根目录路径 (或直接退出): "
                        ).strip()
                        if resp:
                            chosen_path_str = resp
                        else:
                            print("未指定客户端路径，退出。")
                            sys.exit(1)
                    except (EOFError, KeyboardInterrupt):
                        print("未指定客户端路径，退出。")
                        sys.exit(1)

    # 规范化游戏客户端目录 (如果是文件如 DFO.exe，则取其父目录)
    p = pathlib.Path(chosen_path_str).resolve()
    if p.is_file():
        client_dir = p.parent
    else:
        client_dir = p
    client_dir_posix = client_dir.as_posix()
    print(f"\n[设定] 游戏客户端目录: {client_dir_posix}")

    # 验证游戏客户端完整性
    dfo_exe = client_dir / "DFO.exe"
    script_pvf = client_dir / "Script.pvf"
    sk_dat = client_dir / "sk.dat"
    check_ok = True
    for req_file, name in [
        (dfo_exe, "DFO.exe"),
        (script_pvf, "Script.pvf"),
        (sk_dat, "sk.dat"),
    ]:
        if req_file.is_file():
            print(f"  [✓] 发现客户端核心文件: {name}")
        else:
            print(f"  [!] 缺失文件: {name} (启动客户端时可能需要该文件)")
            check_ok = False

    if check_ok:
        print("  --> 客户端核心文件验证全部通过。")
    else:
        print("  --> 提示: 请确认选择的是完整的 115 级 DFO 客户端安装目录。")

    # 1. 更新 server/launcher.local.json
    launcher_cfg["client_dir"] = client_dir_posix
    if args.server_binary:
        launcher_cfg["server_binary"] = args.server_binary
    elif "server_binary" not in launcher_cfg or not launcher_cfg["server_binary"]:
        launcher_cfg["server_binary"] = "work/dfo-lan/bin/wireprobe-handoff-source.exe"

    launcher_file.parent.mkdir(parents=True, exist_ok=True)
    launcher_file.write_text(
        json.dumps(launcher_cfg, indent=2, ensure_ascii=False) + "\n", encoding="utf-8"
    )
    print(f"\n[写入成功] {launcher_file}")
    print(f"  - client_dir    = {launcher_cfg['client_dir']}")
    print(f"  - server_binary = {launcher_cfg['server_binary']}")

    # 2. 更新 server/work/dfo-lan/runtime/storage/local.json
    storage_cfg = {}
    if storage_file.exists():
        try:
            storage_cfg = json.loads(storage_file.read_text(encoding="utf-8-sig"))
            print(f"[加载] 现有存储配置: {storage_file}")
        except Exception as exc:
            print(f"[警告] 读取现有 {storage_file} 失败: {exc}")
    elif storage_example.exists():
        try:
            storage_cfg = json.loads(storage_example.read_text(encoding="utf-8-sig"))
            print(f"[加载] 从模板初始化存储配置: {storage_example}")
        except Exception:
            pass

    # 从当前 cwd/root_dir 计算服务端路径
    postgres_bin_path = (root_dir / "tools/pg/pgsql/bin").resolve().as_posix()
    postgres_data_path = (
        (root_dir / "server/work/dfo-lan/runtime/storage/pgdata").resolve().as_posix()
    )

    # Keep existing PostgreSQL credentials and settings; discard retired cache keys.
    storage_cfg = {
        key: value for key, value in storage_cfg.items()
        if not key.startswith("redis_")
    }
    # Only a PostgreSQL profile needs a pointer to the portable server. A sqlite profile
    # keeps its database in a file, and writing these would leave a stale reference to
    # tools/pg - the kind of leftover that keeps a removed tool tree looking required.
    driver = storage_profile.storage_driver(storage_cfg)
    if driver == "postgres":
        storage_cfg["postgres_bin"] = postgres_bin_path
        storage_cfg["postgres_data"] = postgres_data_path
        # A leftover sqlite_path in a PostgreSQL profile is not a harmless extra key: with
        # no explicit driver it used to make the server open (and create) a SQLite file
        # while the launcher started PostgreSQL, so the player's account looked missing.
        # The key is now only a fallback selector, but a stale one still misleads the
        # next reader of this file, so it is dropped and reported.
        dropped = [key for key in ("sqlite_path", "sqlite_busy_timeout_ms", "busy_timeout_ms", "max_read_connections")
                   if key in storage_cfg]
        for key in dropped:
            storage_cfg.pop(key, None)
        if dropped:
            print(f"[清理] PostgreSQL 档不再保留 SQLite 专有键: {', '.join(dropped)}")
    else:
        storage_cfg.pop("postgres_bin", None)
        storage_cfg.pop("postgres_data", None)

    # 保证必要配置项有合理默认值
    # 默认 DSN 只写给 PostgreSQL 档：SQLite 档拿着它没有用处，而且一旦该档没有显式
    # driver，多出来的 DSN 会把下一次启动的选择翻成 PostgreSQL（见 storage_profile）。
    if driver == "postgres" and "postgres_dsn" not in storage_cfg:
        storage_cfg["postgres_dsn"] = (
            "postgres://dfo_owner:-J5vg5kBCfjt5WccbR1OkkXChKXxxqOBt1mZF6spNDI@127.0.0.1:25438/dfo_lan?sslmode=disable"
        )
    if "max_connections" not in storage_cfg:
        storage_cfg["max_connections"] = 12

    storage_file.parent.mkdir(parents=True, exist_ok=True)
    storage_file.write_text(
        json.dumps(storage_cfg, indent=2, ensure_ascii=False) + "\n", encoding="utf-8"
    )
    print(f"\n[写入成功] {storage_file}")
    if driver == "postgres":
        print(f"  - postgres_bin  = {storage_cfg['postgres_bin']}")
        print(f"  - postgres_data = {storage_cfg['postgres_data']}")
    else:
        print(f"  - driver        = {storage_cfg.get('driver')}")
        print(f"  - sqlite_path   = {storage_cfg.get('sqlite_path')}")

    # 验证服务端依赖程序是否存在：只有 PostgreSQL 档才需要便携 PostgreSQL。
    if driver == "postgres":
        print("\n[检查] 服务端依赖项验证:")
        pg_ctl_exe = pathlib.Path(postgres_bin_path) / "pg_ctl.exe"
        pg_data_dir = pathlib.Path(postgres_data_path)

        print(f"  [{'✓' if pg_ctl_exe.is_file() else '!'}] PostgreSQL pg_ctl: {pg_ctl_exe}")
        print(
            f"  [{'✓' if (pg_data_dir / 'PG_VERSION').is_file() else '!'}] PostgreSQL 数据文件 (PG_VERSION): {pg_data_dir / 'PG_VERSION'}"
        )
    else:
        print("\n[检查] SQLite 档不依赖便携 PostgreSQL；数据库文件:")
        sqlite_file = pathlib.Path(str(storage_cfg.get("sqlite_path", "") or ""))
        if str(sqlite_file) and not sqlite_file.is_absolute():
            sqlite_file = root_dir / sqlite_file
        print(
            f"  [{'✓' if str(sqlite_file) and sqlite_file.is_file() else '!'}] SQLite 库文件: {sqlite_file or '(未配置 sqlite_path)'}"
        )

    print("\n" + "=" * 60)
    print(" 环境配置完成！您现在可以通过 'scripts\启动服务端.cmd' 或 'scripts\启动游戏.cmd' 运行。")
    print("=" * 60)


if __name__ == "__main__":
    main()

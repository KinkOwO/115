@echo off
chcp 65001 >nul
cd /d "%~dp0.."
title DFO 115us 启动（PostgreSQL 路线）

rem 双库双路线入口之一：把活动存储档切成 PostgreSQL 路线，再把 PostgreSQL 拉起来，
rem 最后交给统一入口 启动游戏.cmd（提权、探针、客户端、服务端都在那里）。
rem 另一条路线见 启动游戏-SQLite.cmd；两条路线的存档互相独立。
echo [路线] PostgreSQL（存档在 PostgreSQL 库；活动档 = runtime\storage\local.postgres.json）
powershell -NoProfile -ExecutionPolicy Bypass -File "scripts\存储档.ps1" use postgres
if errorlevel 1 (
    echo.
    echo 路线切换失败：没有启动任何程序。修好上面的问题再重试。
    pause
    exit /b 1
)

rem PostgreSQL 起库：Go 启动器的 start-storage 按 driver 分叉，SQLite 档下它是空操作。
rem 这里失败不直接退出——统一入口还会自己处理并给出更具体的错误。
if exist "server\work\dfo-lan\bin\dfolauncher.exe" (
    echo [存储] 检查并拉起 PostgreSQL...
    "server\work\dfo-lan\bin\dfolauncher.exe" start-storage --root "%~dp0.."
)

echo.
echo [启动] 调用统一入口 启动游戏.cmd ...
call "%~dp0启动游戏.cmd" %*
exit /b %ERRORLEVEL%

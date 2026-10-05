@echo off
chcp 65001 >nul
cd /d "%~dp0.."
title DFO 115us 启动（SQLite 路线）

rem 双库双路线入口之一：把活动存储档切成 SQLite 路线（单文件、不需要 PostgreSQL），
rem 再交给统一入口 启动游戏.cmd。另一条路线见 启动游戏-PostgreSQL.cmd。
rem 两条路线的存档互相独立：SQLite 库文件 ↔ PostgreSQL 库。
echo [路线] SQLite（存档在 runtime\storage\dfolan.sqlite3；活动档 = runtime\storage\local.sqlite.json）
powershell -NoProfile -ExecutionPolicy Bypass -File "scripts\存储档.ps1" use sqlite
if errorlevel 1 (
    echo.
    echo 路线切换失败：没有启动任何程序。修好上面的问题再重试。
    pause
    exit /b 1
)

echo.
echo [启动] 调用统一入口 启动游戏.cmd ...
call "%~dp0启动游戏.cmd" %*
exit /b %ERRORLEVEL%

@echo off
chcp 65001 >nul
cd /d "%~dp0.."
title DFO 115us 服务端（SQLite 路线）

rem 只起服务端（不开客户端）的 SQLite 路线入口；游戏全链见 启动游戏-SQLite.cmd。
echo [路线] SQLite（存档在 runtime\storage\dfolan.sqlite3；活动档 = runtime\storage\local.sqlite.json）
powershell -NoProfile -ExecutionPolicy Bypass -File "scripts\存储档.ps1" use sqlite
if errorlevel 1 (
    echo.
    echo 路线切换失败：没有启动任何程序。修好上面的问题再重试。
    pause
    exit /b 1
)

echo.
echo [启动] 调用统一入口 启动服务端.cmd ...
call "%~dp0启动服务端.cmd" %*
exit /b %ERRORLEVEL%

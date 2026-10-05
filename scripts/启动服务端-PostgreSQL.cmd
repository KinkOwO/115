@echo off
chcp 65001 >nul
cd /d "%~dp0.."
title DFO 115us 服务端（PostgreSQL 路线）

rem 只起服务端（不开客户端）的 PostgreSQL 路线入口；游戏全链见 启动游戏-PostgreSQL.cmd。
echo [路线] PostgreSQL（存档在 PostgreSQL 库；活动档 = runtime\storage\local.postgres.json）
powershell -NoProfile -ExecutionPolicy Bypass -File "scripts\存储档.ps1" use postgres
if errorlevel 1 (
    echo.
    echo 路线切换失败：没有启动任何程序。修好上面的问题再重试。
    pause
    exit /b 1
)

if exist "server\work\dfo-lan\bin\dfolauncher.exe" (
    echo [存储] 检查并拉起 PostgreSQL...
    "server\work\dfo-lan\bin\dfolauncher.exe" start-storage --root "%~dp0.."
)

echo.
echo [启动] 调用统一入口 启动服务端.cmd ...
call "%~dp0启动服务端.cmd" %*
exit /b %ERRORLEVEL%

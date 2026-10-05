@echo off
chcp 65001 >nul
cd /d "%~dp0.."
title DFO 115us 存储路线

rem 双库双路线的切换/查看入口（薄封装；逻辑在 scripts\存储档.ps1）。
rem   存储档.cmd                 查看当前路线
rem   存储档.cmd use sqlite      切到 SQLite 路线
rem   存储档.cmd use postgres    切到 PostgreSQL 路线
rem   存储档.cmd stop-postgres   停掉本仓库 pgdata 上的 PostgreSQL
rem   存储档.cmd selftest        自检（不动本机真实配置）
if "%~1"=="" (
    powershell -NoProfile -ExecutionPolicy Bypass -File "scripts\存储档.ps1" show
) else (
    powershell -NoProfile -ExecutionPolicy Bypass -File "scripts\存储档.ps1" %*
)
set "RC=%ERRORLEVEL%"
echo.
pause
exit /b %RC%

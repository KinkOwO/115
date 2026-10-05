@echo off
chcp 65001 >nul
cd /d "%~dp0.."
rem Dual-storage route helper. All non-ASCII text lives in scripts\storage-route.ps1
rem (UTF-8 with BOM, which PowerShell decodes correctly). This batch file keeps every
rem executable line pure ASCII on purpose: cmd.exe decodes batch text with the console
rem code page, so UTF-8 Chinese inside an executable line can break the parser
rem (2026-10-05: "'hell' is not recognized as an internal or external command").
rem
rem   storage-route.cmd                 show the current route and help
rem   storage-route.cmd show            current route / which database it opens
rem   storage-route.cmd use sqlite      switch the active profile to SQLite
rem   storage-route.cmd use postgres    switch the active profile to PostgreSQL
rem   storage-route.cmd stop-postgres   stop the PostgreSQL instance in this repo
rem   storage-route.cmd clear-guard     drop a stale SQLite admin lease (server refused to start)
rem   storage-route.cmd selftest        self-check in a temp dir, touches nothing real
if "%~1"=="" (
    powershell -NoProfile -ExecutionPolicy Bypass -File "scripts\storage-route.ps1" help
) else (
    powershell -NoProfile -ExecutionPolicy Bypass -File "scripts\storage-route.ps1" %*
)
set "RC=%ERRORLEVEL%"
echo.
pause
exit /b %RC%

@echo off
chcp 65001 >nul
cd /d "%~dp0.."
rem Route: PostgreSQL (local instance in runtime\storage\pgdata, port 25438).
rem This file is deliberately pure ASCII: the Chinese messages and the Chinese-named
rem entry scripts are handled inside scripts\storage-route.ps1 (UTF-8 with BOM), because
rem cmd.exe decodes batch text with the console code page and UTF-8 Chinese inside an
rem executable line breaks the parser on some consoles.
rem Route profiles: runtime\storage\local.sqlite.json / local.postgres.json
rem See server\work\dfo-lan\docs\sqlite-operations.md section 1.2.
powershell -NoProfile -ExecutionPolicy Bypass -File "scripts\storage-route.ps1" game-postgres %*
set "RC=%ERRORLEVEL%"
if "%RC%"=="3" (
    echo.
    echo Route switch failed - nothing was launched. See the message above.
    pause
)
exit /b %RC%

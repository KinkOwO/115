@echo off
chcp 65001 >nul
cd /d "%~dp0.."
rem Route: SQLite, server only (no game client). Full game chain: the game entry.
rem Pure ASCII on purpose; Chinese messages live in scripts\storage-route.ps1 (BOM).
rem See server\work\dfo-lan\docs\sqlite-operations.md section 1.2.
powershell -NoProfile -ExecutionPolicy Bypass -File "scripts\storage-route.ps1" server-sqlite %*
set "RC=%ERRORLEVEL%"
if "%RC%"=="3" (
    echo.
    echo Route switch failed - nothing was launched. See the message above.
    pause
)
exit /b %RC%

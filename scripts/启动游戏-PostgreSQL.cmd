@echo off
chcp 65001 >nul
cd /d "%~dp0.."
title DFO 115us - PostgreSQL route

rem Standalone route entry: storage profile -> PostgreSQL (start it if needed) -> full chain
rem (probe + server + game client). Pure ASCII on purpose (see AGENTS.md 0.4.2);
rem Chinese messages come from scripts\storage-route.ps1, which also does the elevation.
rem Elevation must NOT be done here: routing this file's Chinese name through
rem cmd -> powershell -Command decodes the UTF-8 bytes as GBK, so the elevated cmd looked
rem for a mojibake path, found nothing and the window closed instantly (2026-10-05).
rem The window stays open on any failure so the reason is readable.

powershell -NoProfile -ExecutionPolicy Bypass -File "scripts\storage-route.ps1" game-postgres %*
set "RC=%ERRORLEVEL%"
if not "%RC%"=="0" (
    echo.
    echo Entry failed with exit code %RC%. See the message above.
    echo Full log: runtime\storage\entry-game-postgres.log
    pause
)
exit /b %RC%

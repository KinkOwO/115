@echo off
chcp 65001 >nul
cd /d "%~dp0.."
title DFO 115us Server Gateway (server only)

rem Pure ASCII by design (AGENTS.md 0.4.2); the Chinese notes for this entry live in
rem scripts\README.md, messages come from scripts\storage-route.ps1 (UTF-8 with BOM).
rem
rem Server-only entry: same Go chain as the SQLite route entries in scripts\, explicit
rem SQLite route (SQLite is the only storage engine since 2026-10-05; PostgreSQL was
rem removed). Elevation is handled inside storage-route.ps1 - do not elevate here.
rem Equipment-journal craft/transform (CMD2259) switches, all default; see README:
rem   DFO_EQUIPMENT_CRAFT_WINDOW           default window 3937; uncomment for 2145
rem   DFO_EQUIPMENT_CRAFT_EXECUTE_ON       confirm (default) / first / never
rem   DFO_EQUIPMENT_CRAFT_GENERATE_VARIANT default 1; 0 = force setState 3 (client gets stuck)

set DFO_SHOP_OPEN_ALL=1
set DFO_MAX_ITEM_PERIOD=1
set DFO_QUEST_VISIBLE_NPC_RELAX=1
set DFO_QUEST_NPC_DISTANCE_MULTIPLIER=5
rem set DFO_EQUIPMENT_CRAFT_WINDOW=0
rem set DFO_EQUIPMENT_CRAFT_EXECUTE_ON=first
rem set DFO_EQUIPMENT_CRAFT_GENERATE_VARIANT=0

echo Starting DFO 115us Local Server (PVF Direct + SQLite + Game Gateway)...

rem Go only (owner's rule 2026-10-05): in-repo Go launcher, no Python, no external
rem launcher, no fallback.
if not exist "server\work\dfo-lan\bin\dfolauncher.exe" (
    echo ERROR: server\work\dfo-lan\bin\dfolauncher.exe is missing.
    echo From a git clone:  powershell -NoProfile -ExecutionPolicy Bypass -File scripts\configure-env.ps1
    echo                   cd server\work\dfo-lan ^&^& tools\go\bin\go.exe build -trimpath -o bin\dfolauncher.exe .\cmd\dfolauncher
    echo From an unpacked release:  the same setup entry unpacks the prebuilt launcher from tools\server-bin.zip.
    pause
    exit /b 1
)

powershell -NoProfile -ExecutionPolicy Bypass -File "scripts\storage-route.ps1" server-sqlite %*
set "RC=%ERRORLEVEL%"
if not "%RC%"=="0" (
    echo.
    echo Server startup failed ^(exit code %RC%^). See the message above.
    echo Full log: server\work\dfo-lan\runtime\storage\entry-server-sqlite.log
    pause
    exit /b %RC%
)

echo.
pause
exit /b 0

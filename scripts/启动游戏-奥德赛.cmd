@echo off
chcp 65001 >nul
cd /d "%~dp0.."
title DFO 115us Game Launcher (forced Odyssey profile)

rem Pure ASCII by design (AGENTS.md 0.4.2); the Chinese notes for this entry live in
rem scripts\README.md, messages come from scripts\storage-route.ps1 (UTF-8 with BOM).
rem
rem Force the WHOLE profile into Odyssey mode. Normal play does not need this: game mode
rem comes from the per-character save projection (create request options[10], story 0 /
rem odyssey 2), which is what the client reads itself - that is the default game entry
rem in scripts\. Use this entry only when you deliberately want the forced profile
rem (same switch as "force profile" in the launcher settings).
rem
rem Route: game-current, i.e. the currently active storage route is kept and only the
rem full chain is started - exactly like the default game entry. Route switching belongs
rem to the explicit route entries, not to this file.
rem Elevation is handled inside storage-route.ps1 - do not elevate here.

set DFO_ODYSSEY_MODE=1
set DFO_SHOP_OPEN_ALL=1
set DFO_CONTRACT_PURCHASE_CRASH_FIX=1
set DFO_MAX_ITEM_PERIOD=1
set DFO_QUEST_VISIBLE_NPC_RELAX=1
set DFO_QUEST_NPC_DISTANCE_MULTIPLIER=5
rem Retain the local Adventure Elite entry selection; PS handles its isolated chain.
set DFO_ADVENTURE_ELITE=1

echo Starting DFO 115us Game Client and Server (PVF Direct + forced Odyssey profile)...

rem Use the in-repository Go chain; storage-route.ps1 resolves the normal or Elite launcher.
powershell -NoProfile -ExecutionPolicy Bypass -File "scripts\storage-route.ps1" game-current %*
set "RC=%ERRORLEVEL%"
if not "%RC%"=="0" (
    echo.
    echo Game launch failed ^(exit code %RC%^). See the message above.
    echo Full log: server\work\dfo-lan\runtime\storage\entry-game-current.log
    pause
    exit /b %RC%
)

echo.
pause
exit /b 0

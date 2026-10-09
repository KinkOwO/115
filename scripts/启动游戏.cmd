@echo off
chcp 65001 >nul
cd /d "%~dp0.."
title DFO 115us Game Launcher (Scenario Mode)

rem Elevation is done inside scripts\storage-route.ps1 (see its Invoke-ElevatedSelf):
rem doing it here routed this file's Chinese name through cmd -> powershell -Command, which
rem decodes UTF-8 bytes as GBK, so the elevated cmd never found the file (2026-10-05).

set DFO_SHOP_OPEN_ALL=1
rem Game mode comes from the per-character save projection (create request options[10]:
rem story 0 / odyssey 2), which is what the client reads itself. DFO_ODYSSEY_MODE is
rem deliberately not set here; force the whole profile with the Odyssey entry or the
rem launcher setting. Chinese notes for this file live in scripts\README.md.
set DFO_CONTRACT_PURCHASE_CRASH_FIX=1
set DFO_MAX_ITEM_PERIOD=1
set DFO_QUEST_VISIBLE_NPC_RELAX=1
set DFO_QUEST_NPC_DISTANCE_MULTIPLIER=5
rem Default profile: configs/pvf-default.json; story mode kept, --json-mode is the
rem explicit legacy fallback.
echo Starting DFO 115us Game Client and Server (PVF Direct + Scenario Mode)...
rem Go only (owner's rule 2026-10-05): no external launcher, no Python, no fallback.
rem storage-route.ps1 keeps the currently active storage route and calls the in-repo
rem Go launcher with the Go-forced switches; the two route entries are the same path
rem with an explicit route. Chinese notes for this file live in scripts\README.md.
rem DFO_ADVENTURE_ELITE=1 selects isolated candidates in storage-route.ps1.
powershell -NoProfile -ExecutionPolicy Bypass -File "scripts\storage-route.ps1" game-current %*
set "RC=%ERRORLEVEL%"
if not "%RC%"=="0" (
    echo.
    echo Game launch failed. Please inspect logs.
    pause
    exit /b %RC%
)
pause

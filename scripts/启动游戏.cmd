@echo off
chcp 65001 >nul
cd /d "%~dp0.."
title DFO 115us Game Launcher (Scenario Mode)

net session >nul 2>&1
if %errorlevel% neq 0 (
    echo Requesting Administrator privileges for WFP network isolation...
    powershell -Command "Start-Process cmd -ArgumentList '/c \"\"%~f0\" %*\"' -Verb RunAs"
    exit /b
)

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
rem Prefer the Go launcher: the session orchestration is Go, so this path needs no
rem Python runtime. DFO_ROOT is explicit because the launcher sits beside the repository
rem and would otherwise infer the wrong tree.
set "DFO_ROOT=%~dp0.."
rem tools\ was moved out of the repository, so expose the moved Go toolchain on PATH for
rem the source build (serverbuild looks at tools\go first, then PATH).
set "PATH=%~dp0..\..\tools\go\bin;%PATH%"
if exist "..\115us-dfolauncher\bin\dfolauncher-cli.exe" (
    "..\115us-dfolauncher\bin\dfolauncher-cli.exe" --launch %*
) else if exist "tools\python\python.exe" (
    "tools\python\python.exe" "server\work\dfo-lan\scripts\launch_local.py" %*
) else (
    python "server\work\dfo-lan\scripts\launch_local.py" %*
)
if errorlevel 1 (
    echo.
    echo Game launch failed. Please inspect logs.
    pause
    exit /b 1
)
pause

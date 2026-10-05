@echo off
chcp 65001 >nul
cd /d "%~dp0.."
title DFO 115us Server Gateway
rem Default profile: configs/pvf-default.json; --repair-profile selects another one,
rem --json-mode is the explicit legacy fallback.
echo Starting DFO 115us Local Server (PVF Direct + Storage + Game Gateway)...
set DFO_SHOP_OPEN_ALL=1
set DFO_MAX_ITEM_PERIOD=1
set DFO_QUEST_VISIBLE_NPC_RELAX=1
set DFO_QUEST_NPC_DISTANCE_MULTIPLIER=5
rem Equipment-journal craft/transform (CMD2259) response tuning. The default window id
rem is 3937; if the craft window does not open, uncomment the line below (window 2145).
rem set DFO_EQUIPMENT_CRAFT_WINDOW=0
rem When the "generate" request actually runs: confirm (default; the second request of
rem the same operation) / first (run immediately, risky) / never.
rem If generate does nothing and the log has a single 2259, try the line below.
rem set DFO_EQUIPMENT_CRAFT_EXECUTE_ON=first
rem Generated-response sub-branch byte (payload[5]): DEFAULT 1 = success flag only, the
rem window state is left alone; 0 = force setState 3, a state the client never enters
rem and cannot leave (its "switch material" button stops responding - live 2026-09-29).
rem Normally leave this alone.
rem set DFO_EQUIPMENT_CRAFT_GENERATE_VARIANT=0
rem Full Chinese notes for these switches live in scripts\README.md.

rem Prefer the Go launcher: the session orchestration is Go, so this path needs no
rem Python runtime. DFO_ROOT is explicit because the launcher sits beside the repository
rem and would otherwise infer the wrong tree.
set "DFO_ROOT=%~dp0.."
rem tools\ was moved out of the repository, so expose the moved Go toolchain on PATH for
rem the source build (serverbuild looks at tools\go first, then PATH).
set "PATH=%~dp0..\..\tools\go\bin;%PATH%"
if exist "..\115us-dfolauncher\bin\dfolauncher-cli.exe" (
    "..\115us-dfolauncher\bin\dfolauncher-cli.exe" --launch --server-only %*
) else if exist "tools\python\python.exe" (
    "tools\python\python.exe" "server\work\dfo-lan\scripts\launch_local.py" --server-only %*
) else (
    python "server\work\dfo-lan\scripts\launch_local.py" --server-only %*
)
if errorlevel 1 (
    echo.
    echo Server startup encountered an error.
    pause
    exit /b 1
)
echo.
pause

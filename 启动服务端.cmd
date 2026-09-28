@echo off
chcp 65001 >nul
cd /d "%~dp0"
title DFO 115us Server Gateway
echo Starting DFO 115us Local Server (Storage + Game Gateway)...
set DFO_SHOP_OPEN_ALL=1
set DFO_MAX_ITEM_PERIOD=1
set DFO_QUEST_VISIBLE_NPC_RELAX=1
set DFO_QUEST_NPC_DISTANCE_MULTIPLIER=5
if exist "tools\python\python.exe" (
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

@echo off
chcp 65001 >nul
cd /d "%~dp0"
title DFO 115us Game Launcher (Arad Odyssey Mode)

net session >nul 2>&1
if %errorlevel% neq 0 (
    echo Requesting Administrator privileges for WFP network isolation...
    powershell -Command "Start-Process cmd -ArgumentList '/c \"\"%~f0\" %*\"' -Verb RunAs"
    exit /b
)

set DFO_SHOP_OPEN_ALL=1
set DFO_ODYSSEY_MODE=1
set DFO_CONTRACT_PURCHASE_CRASH_FIX=0
set DFO_MAX_ITEM_PERIOD=1
set DFO_QUEST_VISIBLE_NPC_RELAX=1
rem 默认使用 configs/pvf-default.json；保留奥德赛模式，--json-mode 显式回退。
echo Starting DFO 115us Game Client and Server (PVF Direct + Arad Odyssey Mode)...
if exist "tools\python\python.exe" (
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

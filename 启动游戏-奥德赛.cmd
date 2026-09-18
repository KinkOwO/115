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
echo Starting DFO 115us Game Client and Server (Arad Odyssey Mode)...
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

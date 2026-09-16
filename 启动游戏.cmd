@echo off
chcp 65001 >nul
cd /d "%~dp0"
title DFO 115us Game Launcher

net session >nul 2>&1
if %errorlevel% neq 0 (
    echo Requesting Administrator privileges for WFP network isolation...
    powershell -Command "Start-Process cmd -ArgumentList '/c \"\"%~f0\" %*\"' -Verb RunAs"
    exit /b
)

echo Starting DFO 115us Game Client and Server...
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

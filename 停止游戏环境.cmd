@echo off
chcp 65001 >nul
cd /d "%~dp0"
title Stop DFO 115us Environment
echo Stopping DFO 115us Environment...
if exist "tools\python\python.exe" (
    "tools\python\python.exe" "stop_environment.py"
) else (
    python "stop_environment.py"
)
pause

@echo off
chcp 65001 >nul
cd /d "%~dp0.."
title DFO 115us Environment Setup

rem ASCII-only by design: cmd.exe decodes batch text with the console code page, so any
rem non-ASCII byte here can be split into bogus commands (see AGENTS.md 0.4.2). The
rem Chinese report and prompts come from scripts\configure_env.py, which Python decodes
rem correctly as UTF-8.
echo ========================================================
echo          DFO 115us local/LAN server environment setup
echo ========================================================
echo.

if exist "tools\python\python.exe" (
    "tools\python\python.exe" "scripts\configure_env.py" %*
) else (
    python "scripts\configure_env.py" %*
)

if errorlevel 1 (
    echo.
    echo [ERROR] Environment setup did not complete. See the message above.
    pause
    exit /b 1
)

echo.
pause

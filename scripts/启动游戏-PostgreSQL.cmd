@echo off
chcp 65001 >nul
cd /d "%~dp0.."
title DFO 115us - PostgreSQL route

rem Standalone route entry: storage profile -> PostgreSQL (start it if needed) -> full chain
rem (probe + server + game client). It picks the launcher itself, so it does not depend on
rem any other entry script. Pure ASCII on purpose: cmd.exe decodes batch text with the
rem console code page, so non-ASCII in an executable line can break the parser
rem (see AGENTS.md 0.4.2). Chinese messages come from scripts\storage-route.ps1.
rem
rem WFP network isolation needs administrator rights, hence the self-elevation below.
net session >nul 2>&1
if %errorlevel% neq 0 (
    echo Requesting Administrator privileges for WFP network isolation...
    powershell -Command "Start-Process cmd -ArgumentList '/c \"\"%~f0\" %*\"' -Verb RunAs"
    exit /b
)

powershell -NoProfile -ExecutionPolicy Bypass -File "scripts\storage-route.ps1" game-postgres %*
set "RC=%ERRORLEVEL%"
if "%RC%"=="3" (
    echo.
    echo Route switch or storage preflight failed - nothing was launched. See the message above.
    pause
)
exit /b %RC%

@echo off
chcp 65001 >nul
cd /d "%~dp0.."
title DFO 115us Environment Setup

rem Pure ASCII by design: cmd.exe decodes batch text with the console code page, so any
rem non-ASCII byte here (even in a rem line) can be split into bogus commands
rem (AGENTS.md 0.4.2). The Chinese report is printed by scripts\configure-env.ps1,
rem which is UTF-8 with BOM so PowerShell decodes it correctly.
rem
rem What it does: unpacks the runtime packages that ship inside the repository
rem (tools\manifest.json -> go / gopath-mod / server-src / server-configs / server-bin).
rem No Python, no network needed. Idempotent; read-only with -Check.
rem Examples:  (this entry) -Check
rem            (this entry) -Package go,gopath-mod
rem            (this entry) -Force

echo ========================================================
echo          DFO 115us environment bootstrap
echo ========================================================
echo.

powershell -NoProfile -ExecutionPolicy Bypass -File "scripts\configure-env.ps1" %*
set "RC=%ERRORLEVEL%"

if not "%RC%"=="0" (
    echo.
    echo [ERROR] Environment setup did not complete ^(exit code %RC%^). See the message above.
    pause
    exit /b %RC%
)

echo.
pause
exit /b 0

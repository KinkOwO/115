@echo off
rem One-click start for the modkit web page (mods/modkit-web).
rem This batch file is pure ASCII on purpose; every Chinese message lives in
rem scripts\start-modkit.ps1 (UTF-8 with BOM), which PowerShell decodes correctly.
rem Usage: see the header of start-modkit.ps1, or run with -? / -Help.
setlocal
cd /d "%~dp0.."
if /i "%~1"=="-?" goto :help
if /i "%~1"=="-h" goto :help
if /i "%~1"=="--help" goto :help
powershell -NoProfile -ExecutionPolicy Bypass -Command "& '%~dp0start-modkit.ps1' %*"
set RC=%ERRORLEVEL%
if not "%RC%"=="0" (
  echo [modkit] exited with code %RC%
  pause
)
exit /b %RC%

:help
powershell -NoProfile -ExecutionPolicy Bypass -Command "& '%~dp0start-modkit.ps1' -Help"
exit /b 0

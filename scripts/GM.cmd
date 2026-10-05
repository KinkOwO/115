@echo off
chcp 65001 >nul
cd /d "%~dp0.."
title DFO 115us GM
rem The Go toolchain lives outside the repository; building/running cmd/admin and
rem dfo-tool needs it on PATH (serverbuild looks at tools\go first, then PATH).
set "PATH=%~dp0..\..\tools\go\bin;%PATH%"
set "DFO_PVF_ARCHIVE=..\client-build\Script.inner.pvf"
set "PY=%~dp0..\..\tools\python\python.exe"
if not exist "%PY%" set "PY=python"

rem ASCII-only by design (AGENTS.md 0.4.2): the Chinese usage banner is printed by
rem scripts\gm.py help, which Python decodes correctly as UTF-8.
if "%~1"=="" goto :usage
"%PY%" "scripts\gm.py" %*
set "RC=%ERRORLEVEL%"
echo.
pause
exit /b %RC%

:usage
"%PY%" "scripts\gm.py" help
"%PY%" "scripts\gm.py" list
echo.
pause
exit /b 0

@echo off
chcp 65001 >nul
cd /d "%~dp0.."
title DFO 115us GM
rem The Go toolchain lives outside the repository; building/running cmd/admin and
rem dfo-tool needs it on PATH (serverbuild looks at tools\go first, then PATH).
set "PATH=%~dp0..\..\tools\go\bin;%PATH%"
set "DFO_PVF_ARCHIVE=..\client-build\Script.inner.pvf"
rem Python is a GM-tool-only dependency and no longer lives in tools\ (the game launch
rem chain does not need it). Prefer the GM-only copy next to tools\, then PATH python.
set "PY=%~dp0..\..\gm-tool\python\python.exe"
if exist "%PY%" goto :py_ready
set "PY=python"
where python >nul 2>nul
if errorlevel 1 goto :no_python
:py_ready

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

:no_python
echo [ERROR] Python interpreter not found.
echo         Python is a GM-tool-only dependency; the game launch chain does not need it.
echo         Looked for, in order:
echo           1) %~dp0..\..\gm-tool\python\python.exe   (GM-only copy next to tools\)
echo           2) python on PATH
echo         To restore: unzip the portable Python 3.11 (capstone / cryptography / pefile / frida)
echo         into the "gm-tool\python" folder that sits next to the package's "tools" folder,
echo         or put python on PATH.
echo.
pause
exit /b 1

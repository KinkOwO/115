@echo off
chcp 65001 >nul
cd /d "%~dp0.."
title Stop DFO 115us Environment
echo Stopping DFO 115us Environment...

rem Prefer the Go launcher: the stop path no longer needs a Python runtime.
if exist "server\work\dfo-lan\bin\dfolauncher.exe" (
    "server\work\dfo-lan\bin\dfolauncher.exe" stop --root "%~dp0.."
    goto :done
)

set "GO_STOP_RC=1"
if exist "tools\go\bin\go.exe" call :go_stop
if "%GO_STOP_RC%"=="0" goto :done
if exist "tools\go\bin\go.exe" echo Go launcher unavailable or failed; falling back to Python.

if exist "tools\python\python.exe" (
    "tools\python\python.exe" "scripts\stop_environment.py"
) else (
    python "scripts\stop_environment.py"
)

:done
pause
exit /b 0

:go_stop
pushd "server\work\dfo-lan"
"..\..\..\tools\go\bin\go.exe" run ./cmd/dfolauncher stop --root "%~dp0.."
rem Read outside any parenthesised block so this is the command's own exit code.
set "GO_STOP_RC=%ERRORLEVEL%"
popd
exit /b 0

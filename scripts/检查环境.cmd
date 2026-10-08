@echo off
chcp 65001 >nul
cd /d "%~dp0.."
title Check DFO 115us Environment

rem Pure ASCII by design (AGENTS.md 0.4.2); Chinese notes for this entry live in
rem scripts\README.md, messages come from the Go launcher / scripts\storage-route.ps1.
rem
rem READ-ONLY environment check: reports dependencies, starts and stops nothing.
rem   1. prefer the in-repo Go launcher:  server\work\dfo-lan\bin\dfolauncher.exe check --root <repo>
rem   2. else build-and-run it once from source with the unpacked toolchain:
rem      tools\go\bin\go.exe run ./cmd/dfolauncher check --root <repo>
rem      (needs tools\go plus the module cache at tools\gopath; scripts\configure-env.ps1 gives both)
rem   3. neither present -> print how to get one. There is deliberately NO Python fallback:
rem      the Python orchestration (launch_local.py) was removed on 2026-10-05.
rem Exit code = the check's own exit code (0 = ok, non-zero = problems reported).
rem
rem No parenthesised blocks on purpose: inside "if ( ... )" cmd expands %RC% at parse
rem time, so the exit code has to be read on a plain line (the 2026-10-05 entry had to
rem use a call label for the same reason).

set "DFO_ROOT=%~dp0.."
set "RC=1"

if not exist "server\work\dfo-lan\bin\dfolauncher.exe" goto :try_source
"server\work\dfo-lan\bin\dfolauncher.exe" check --root "%DFO_ROOT%" %*
set "RC=%ERRORLEVEL%"
goto :report

:try_source
if not exist "tools\go\bin\go.exe" goto :no_launcher
rem Use the in-repo module cache when it is there, so a fresh clone can check offline.
if exist "tools\gopath\pkg\mod" set "GOPATH=%CD%\tools\gopath"
if exist "tools\gopath\pkg\mod" set "GOMODCACHE=%CD%\tools\gopath\pkg\mod"
if exist "tools\gopath\pkg\mod" set "GOTOOLCHAIN=local"
pushd "server\work\dfo-lan"
"..\..\..\tools\go\bin\go.exe" run ./cmd/dfolauncher check --root "%DFO_ROOT%" %*
set "RC=%ERRORLEVEL%"
popd
goto :report

:no_launcher
echo ERROR: neither server\work\dfo-lan\bin\dfolauncher.exe nor tools\go\bin\go.exe exists.
echo   From a git clone:  powershell -NoProfile -ExecutionPolicy Bypass -File scripts\configure-env.ps1
echo                     cd server\work\dfo-lan ^&^& tools\go\bin\go.exe build -trimpath -o bin\dfolauncher.exe .\cmd\dfolauncher
echo   From an unpacked release: the same setup entry unpacks the prebuilt launcher from tools\server-bin.zip.
set "RC=1"
goto :report

:report
echo.
if not "%RC%"=="0" echo Environment check reported problems ^(exit code %RC%^).
pause
exit /b %RC%

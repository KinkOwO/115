@echo off
chcp 65001 >nul
cd /d "%~dp0.."
title Stop DFO 115us Environment
echo Stopping DFO 115us Environment...

rem Go only (owner's rule 2026-10-05): no Python, no source-build fallback.
rem The Go launcher stops the game server/probes and, on a PostgreSQL profile, the
rem database; on a SQLite profile it also clears a lease whose holder is gone.
rem Chinese notes for this file live in scripts\README.md.
if not exist "server\work\dfo-lan\bin\dfolauncher.exe" (
    echo ERROR: server\work\dfo-lan\bin\dfolauncher.exe is missing.
    echo Build it once on a dev machine:  cd server\work\dfo-lan ^&^& go build -trimpath -o bin\dfolauncher.exe .\cmd\dfolauncher
    pause
    exit /b 1
)
"server\work\dfo-lan\bin\dfolauncher.exe" stop --root "%~dp0.."
set "RC=%ERRORLEVEL%"

pause
exit /b %RC%

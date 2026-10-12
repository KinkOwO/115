@echo off
chcp 65001 >nul
cd /d "%~dp0.."
title Stop DFO 115us Environment
echo Stopping DFO 115us Environment...

rem Elevation and SQLite lease checks share the storage entry's Unicode-safe handoff.
powershell -NoProfile -ExecutionPolicy Bypass -File "scripts\storage-route.ps1" stop-environment %*
set "RC=%ERRORLEVEL%"

pause
exit /b %RC%

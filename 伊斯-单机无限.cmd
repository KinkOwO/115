@echo off
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0server\work\dfo-lan\scripts\Set-Ispins-Mode.ps1" -Mode unlimited
pause

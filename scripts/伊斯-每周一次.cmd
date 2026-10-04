@echo off
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0..\server\work\dfo-lan\scripts\Set-Ispins-Mode.ps1" -Mode weekly
pause

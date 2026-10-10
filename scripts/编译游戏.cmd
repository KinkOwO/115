@echo off
chcp 65001 >nul
cd /d "%~dp0.."
title DFO 115us - Build launcher and source server
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "scripts\build-game.ps1" %*
set "RC=%ERRORLEVEL%"
echo.
if not "%RC%"=="0" echo Build failed with exit code %RC%. See the message above.
pause
exit /b %RC%
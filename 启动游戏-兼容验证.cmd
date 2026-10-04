@echo off
setlocal
chcp 65001 >nul
title DFO Client Compatibility Validation
set "DFO_VALIDATION_ROOT=%~dp0"
set "DFO_VALIDATION_SELF=%~f0"
echo DFO town entry validation
if /i "%~1"=="--check" goto run
net session >nul 2>&1
if not errorlevel 1 goto run
if /i "%~1"=="--elevated" goto elevation_failed
echo Requesting Administrator privileges...
powershell -NoProfile -Command "try { Start-Process -FilePath $env:ComSpec -ArgumentList ('/d /k '+[char]34+[char]34+$env:DFO_VALIDATION_SELF+[char]34+' --elevated'+[char]34) -Verb RunAs -ErrorAction Stop } catch { Write-Host $_; exit 1 }"
if errorlevel 1 goto elevation_failed
exit /b 0
:run
cd /d "%DFO_VALIDATION_ROOT%"
if errorlevel 1 goto failed
set DFO_SHOP_OPEN_ALL=1
set DFO_ODYSSEY_MODE=0
set DFO_CONTRACT_PURCHASE_CRASH_FIX=1
set DFO_MAX_ITEM_PERIOD=1
set DFO_QUEST_VISIBLE_NPC_RELAX=1
set DFO_QUEST_NPC_DISTANCE_MULTIPLIER=5
echo Workspace: %CD%
if not exist "tools\python\python.exe" goto failed
if not exist "server\work\dfo-lan\.tmp\compat-rebuild-20261004\profile.json" goto failed
if /i "%~1"=="--check" goto check
"tools\python\python.exe" "server\work\dfo-lan\scripts\launch_local.py" --repair-profile "%DFO_VALIDATION_ROOT%\server\work\dfo-lan\.tmp\compat-rebuild-20261004\profile.json"
if errorlevel 1 goto failed
echo Validation session ended.
pause
exit /b 0
:check
"tools\python\python.exe" "server\work\dfo-lan\scripts\launch_local.py" --repair-profile "%DFO_VALIDATION_ROOT%\server\work\dfo-lan\.tmp\compat-rebuild-20261004\profile.json" --check
exit /b %errorlevel%
:elevation_failed
echo Administrator launch failed or was cancelled. Right-click this file and run as administrator.
pause
exit /b 1
:failed
echo Validation startup failed. Read the error above.
pause
exit /b 1

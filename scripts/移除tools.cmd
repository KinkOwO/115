@echo off
chcp 65001 >nul
cd /d "%~dp0.."
title Move the portable toolchain aside

rem Move, never delete: tools is relocated outside the repository and can be brought
rem back with the restore entry at any time. Chinese notes live in scripts\README.md.
set "SRC=%~1"
if "%SRC%"=="" set "SRC=tools"
set "DST=%~2"
if "%DST%"=="" set "DST=..\115-tools-moved"

if not exist "%SRC%" (
    echo Nothing to move: "%SRC%" does not exist.
    pause
    exit /b 1
)
if exist "%DST%" (
    echo Refusing to move: "%DST%" already exists. Move or rename it first.
    pause
    exit /b 1
)

echo.
echo Moving "%SRC%" to "%DST%".
echo After the move:
echo   * the check and stop entries keep working: they use server\work\dfo-lan\bin\dfolauncher.exe
echo   * the game / server entries still need the Python orchestration (channel_probe.py);
echo     see docs/runtime-without-tools-plan.md before expecting the game to start.
echo   * A PostgreSQL profile loses its portable server; a sqlite profile does not need one.
echo.
move "%SRC%" "%DST%" >nul
if errorlevel 1 (
    echo Move FAILED.
    pause
    exit /b 1
)
echo Moved. Restore with the restore-tools entry in scripts\.
pause

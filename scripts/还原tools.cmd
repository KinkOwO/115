@echo off
chcp 65001 >nul
cd /d "%~dp0.."
title Restore the portable toolchain

set "DST=%~1"
if "%DST%"=="" set "DST=..\115-tools-moved"
set "SRC=%~2"
if "%SRC%"=="" set "SRC=tools"

if not exist "%DST%" (
    echo Nothing to restore: "%DST%" does not exist.
    pause
    exit /b 1
)
if exist "%SRC%" (
    echo Refusing to restore: "%SRC%" already exists. Move it away first.
    pause
    exit /b 1
)

move "%DST%" "%SRC%" >nul
if errorlevel 1 (
    echo Restore FAILED.
    pause
    exit /b 1
)
echo Restored "%SRC%" from "%DST%".
pause

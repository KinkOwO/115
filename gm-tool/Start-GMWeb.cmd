@echo off
chcp 65001 >nul
setlocal
cd /d "%~dp0"
if exist "D:\115us\tools\python\python.exe" (
    "D:\115us\tools\python\python.exe" "%~dp0scripts\gmweb.py" %*
) else (
    "%~dp0python\python.exe" "%~dp0scripts\gmweb.py" %*
)
if errorlevel 1 (
    echo GM web startup failed. Read the error above.
    pause
    exit /b 1
)
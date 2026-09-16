@echo off
setlocal
title DFO Local Test Launcher
"C:\Users\Administrator\.cache\codex-runtimes\codex-primary-runtime\dependencies\python\python.exe" "%~dp0work\dfo-lan\scripts\launch_local.py" %*
if errorlevel 1 (
  echo.
  echo Startup failed. Please read the error above.
  pause
  exit /b 1
)
timeout /t 5 >nul

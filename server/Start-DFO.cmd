@echo off
setlocal
cd /d "%~dp0"
if defined DFO_PYTHON (
  "%DFO_PYTHON%" "%~dp0work\dfo-lan\scripts\launch_local.py" %*
) else (
  where py >nul 2>nul
  if not errorlevel 1 (
    py -3 "%~dp0work\dfo-lan\scripts\launch_local.py" %*
  ) else (
    python "%~dp0work\dfo-lan\scripts\launch_local.py" %*
  )
)
if errorlevel 1 (
  echo Startup failed. Read the error above.
  pause
  exit /b 1
)
pause

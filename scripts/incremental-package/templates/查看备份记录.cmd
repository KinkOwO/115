@echo off
setlocal
cd /d "%~dp0"
set "PSEXE=powershell"
where pwsh >nul 2>nul && set "PSEXE=pwsh"
"%PSEXE%" -NoProfile -ExecutionPolicy Bypass -File "%~dp0_update\update.ps1" -Mode List %*
echo.
pause
exit /b %errorlevel%

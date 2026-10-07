@echo off
setlocal
cd /d "%~dp0"
set "PSEXE=powershell"
where pwsh >nul 2>nul && set "PSEXE=pwsh"
echo === DFO 115us incremental update: INSTALL ===
echo.
"%PSEXE%" -NoProfile -ExecutionPolicy Bypass -File "%~dp0_update\update.ps1" -Mode Apply %*
set "RC=%errorlevel%"
echo.
if "%RC%"=="0" (echo [DONE] Incremental update installed.) else (echo [FAILED] exit code %RC%)
echo.
pause
exit /b %RC%

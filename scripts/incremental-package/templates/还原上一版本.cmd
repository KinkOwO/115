@echo off
setlocal
cd /d "%~dp0"
set "PSEXE=powershell"
where pwsh >nul 2>nul && set "PSEXE=pwsh"
echo === DFO 115us incremental update: RESTORE (one click rollback) ===
echo.
"%PSEXE%" -NoProfile -ExecutionPolicy Bypass -File "%~dp0_update\update.ps1" -Mode Restore %*
set "RC=%errorlevel%"
echo.
if "%RC%"=="0" (echo [DONE] Restored to the state before the update.) else (echo [FAILED] exit code %RC%)
echo.
pause
exit /b %RC%

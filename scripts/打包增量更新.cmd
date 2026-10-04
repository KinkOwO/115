@echo off
setlocal
cd /d "%~dp0.."
set "PSARGS=%*"
if "%PSARGS%"=="" set "PSARGS=-Commit HEAD"
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0incremental-package\build-incremental.ps1" %PSARGS%
set "RC=%errorlevel%"
echo.
if not "%RC%"=="0" pause
exit /b %RC%

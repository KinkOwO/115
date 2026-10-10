@echo off
chcp 65001 >nul
setlocal
cd /d "%~dp0.."
set "BAKAL_LAUNCHER=server\work\dfo-lan\bin\dfolauncher-bakal-compat-candidate.exe"
if not exist "%BAKAL_LAUNCHER%" (
    echo Bakal-compatible launcher is missing. Build cmd/dfolauncher first.
    exit /b 1
)
"%BAKAL_LAUNCHER%" bakal-reset --root "%CD%" --apply %*
exit /b %ERRORLEVEL%

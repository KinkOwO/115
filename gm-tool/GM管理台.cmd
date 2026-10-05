@echo off
chcp 65001 >nul
setlocal enabledelayedexpansion

REM ============================================================
REM  DFO 115us GM Console - Unified Launcher
REM  Starts gmweb.exe (28080) + Python proxy (28081)
REM  Logs to logs\ directory, keeps 7 days
REM ============================================================

set GMTOOL=%~dp0
set LOGDIR=%GMTOOL%logs
rem Python is a GM-tool-only dependency (the game launch chain does not need it).
rem Resolution order: in-package GM dir -> GM-only copy next to tools\ -> python on PATH.
set PYEXE=%GMTOOL%python\python.exe
if not exist "%PYEXE%" set PYEXE=%GMTOOL%..\..\gm-tool\python\python.exe
set PYEXE_OK=1
if not exist "%PYEXE%" (
    set PYEXE=python
    where python >nul 2>nul
    if errorlevel 1 set PYEXE_OK=0
)

REM ---------- Prepare log directory and date ----------
if not exist "%LOGDIR%" mkdir "%LOGDIR%"
REM Use PowerShell for date (YYYYMMDD), independent of system locale
for /f "usebackq tokens=*" %%a in (`powershell -NoProfile -Command "Get-Date -Format yyyyMMdd"`) do set TODAY=%%a
set PROXYLOG=%LOGDIR%\proxy_%TODAY%.log
set GMWEBLOG=%LOGDIR%\gmweb_%TODAY%.log

REM Clean old logs older than 7 days
if exist "%LOGDIR%\*.log" forfiles /p "%LOGDIR%" /m *.log /d -7 /c "cmd /c del @path" >nul 2>&1

echo === DFO GM Console Starting ===
echo Logs: %LOGDIR%
echo.

REM ---------- Step 1: Start gmweb.exe (28080) ----------
netstat -ano | findstr ":28080 " | findstr LISTENING >nul
if %errorlevel%==0 (
    echo [OK] gmweb.exe already running on 28080
) else (
    echo [..] Starting gmweb.exe ...
        start "" /b cmd /c ""%GMTOOL%Start-GMWeb.cmd" --no-browser >> "%GMWEBLOG%" 2>&1"
    set WAIT_OK=0
    for /l %%i in (1,1,20) do (
        if !WAIT_OK!==0 (
            timeout /t 1 /nobreak >nul
            netstat -ano | findstr ":28080 " | findstr LISTENING >nul
            if !errorlevel!==0 set WAIT_OK=1
        )
    )
    if !WAIT_OK!==0 (
        echo [FAIL] gmweb.exe timeout, not listening on 28080
        echo Check log: %GMWEBLOG%
        pause
        exit /b 1
    )
    echo [OK] gmweb.exe started on 28080
    echo      Log: %GMWEBLOG%
)

REM ---------- Step 2: Start Python proxy (28081) ----------
if "%PYEXE_OK%"=="0" goto :no_python
netstat -ano | findstr ":28081 " | findstr LISTENING >nul
if %errorlevel%==0 (
    echo [OK] Python proxy already running on 28081
) else (
    echo [..] Starting Python proxy ...
        start "" /b cmd /c ""%PYEXE%" -u "%GMTOOL%dashboard\gm_dashboard_proxy.py" >> "%PROXYLOG%" 2>&1"
    set WAIT_OK=0
    for /l %%i in (1,1,15) do (
        if !WAIT_OK!==0 (
            timeout /t 1 /nobreak >nul
            netstat -ano | findstr ":28081 " | findstr LISTENING >nul
            if !errorlevel!==0 set WAIT_OK=1
        )
    )
    if !WAIT_OK!==0 (
        echo [FAIL] Python proxy timeout, not listening on 28081
        echo Check log: %PROXYLOG%
        pause
        exit /b 1
    )
    echo [OK] Python proxy started on 28081
    echo      Log: %PROXYLOG%
)

REM ---------- Step 3: Open browser ----------
echo.
echo === Startup Complete ===
echo   Console: http://127.0.0.1:28081/
echo   Backend: http://127.0.0.1:28080/
echo.
start "" "http://127.0.0.1:28081/"

endlocal
exit /b 0

:no_python
echo [ERROR] Python interpreter not found.
echo         Python is a GM-tool-only dependency; the game launch chain does not need it.
echo         Looked for, in order:
echo           1) %GMTOOL%python\python.exe
echo           2) %GMTOOL%..\..\gm-tool\python\python.exe   (GM-only copy next to tools\)
echo           3) python on PATH
echo         To restore: unzip the portable Python 3.11 (capstone / cryptography / pefile / frida)
echo         into the "gm-tool\python" folder that sits next to the package's "tools" folder,
echo         or put python on PATH.
echo.
pause
exit /b 1

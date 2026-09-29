@echo off
chcp 65001 >nul
setlocal enabledelayedexpansion

REM ============================================================
REM  DFO 115us GM Console - Stop Script
REM  Stops gmweb.exe and Python proxy, verifies ports released
REM ============================================================

echo === Stopping DFO GM Console ===

REM ---------- Stop Python proxy (28081) ----------
for /f "tokens=5" %%p in ('netstat -ano ^| findstr ":28081 " ^| findstr LISTENING') do (
    tasklist /FI "PID eq %%p" 2>nul | findstr /I "python" >nul
    if !errorlevel!==0 (
        echo [..] Stopping Python proxy PID=%%p
        taskkill /PID %%p /F 2>nul
    ) else (
        echo [SKIP] PID=%%p is not python, not killing
    )
)

REM ---------- Stop gmweb.exe (28080) ----------
for /f "tokens=5" %%p in ('netstat -ano ^| findstr ":28080 " ^| findstr LISTENING') do (
    tasklist /FI "PID eq %%p" 2>nul | findstr /I "gmweb" >nul
    if !errorlevel!==0 (
        echo [..] Stopping gmweb.exe PID=%%p
        taskkill /PID %%p /F 2>nul
    ) else (
        echo [SKIP] PID=%%p is not gmweb, not killing
    )
)

timeout /t 2 /nobreak >nul

REM ---------- Verify ports released ----------
netstat -ano | findstr ":28080 " | findstr LISTENING >nul && echo [WARN] 28080 still in use || echo [OK] 28080 released
netstat -ano | findstr ":28081 " | findstr LISTENING >nul && echo [WARN] 28081 still in use || echo [OK] 28081 released

echo.
pause
endlocal

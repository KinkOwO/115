@echo off
chcp 65001 >nul
cd /d "%~dp0.."
title DFO 115us 环境配置向导

echo ========================================================
echo          DFO 115us 本地/局域网服务端环境配置
echo ========================================================
echo.

if exist "tools\python\python.exe" (
    "tools\python\python.exe" "scripts\configure_env.py" %*
) else (
    python "scripts\configure_env.py" %*
)

if errorlevel 1 (
    echo.
    echo [错误] 环境配置未能成功完成。
    pause
    exit /b 1
)

echo.
pause

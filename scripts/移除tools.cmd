@echo off
chcp 65001 >nul
cd /d "%~dp0.."
title Move the portable toolchain aside

rem 移动而不是删除：只是把 tools 挪到仓库外，随时可用 scripts\还原tools.cmd 移回来。
set "SRC=%~1"
if "%SRC%"=="" set "SRC=tools"
set "DST=%~2"
if "%DST%"=="" set "DST=..\115-tools-moved"

if not exist "%SRC%" (
    echo Nothing to move: "%SRC%" does not exist.
    pause
    exit /b 1
)
if exist "%DST%" (
    echo Refusing to move: "%DST%" already exists. Move or rename it first.
    pause
    exit /b 1
)

echo.
echo Moving "%SRC%" to "%DST%".
echo After the move:
echo   * scripts\检查环境.cmd and scripts\停止游戏环境.cmd keep working: they use server\work\dfo-lan\bin\dfolauncher.exe
echo   * scripts\启动游戏.cmd / scripts\启动服务端.cmd still need the Python orchestration (channel_probe.py);
echo     see docs/runtime-without-tools-plan.md before expecting the game to start.
echo   * A PostgreSQL profile loses its portable server; a sqlite profile does not need one.
echo.
move "%SRC%" "%DST%" >nul
if errorlevel 1 (
    echo Move FAILED.
    pause
    exit /b 1
)
echo Moved. Restore with: scripts\还原tools.cmd
pause

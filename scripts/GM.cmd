@echo off
chcp 65001 >nul
cd /d "%~dp0.."
title DFO 115us GM
rem Go 工具链已移出仓库，构建/运行 cmd/admin、dfo-tool 需要它在 PATH 上（serverbuild 同样先看 PATH）。
set "PATH=%~dp0..\..\tools\go\bin;%PATH%"
set "DFO_PVF_ARCHIVE=..\client-build\Script.inner.pvf"
set "PY=%~dp0..\..\tools\python\python.exe"
if not exist "%PY%" set "PY=python"

if "%~1"=="" goto :usage
"%PY%" "scripts\gm.py" %*
set "RC=%ERRORLEVEL%"
echo.
pause
exit /b %RC%

:usage
"%PY%" "scripts\gm.py" list
echo.
echo ============================================================
echo  DFO 115us 简易 GM（SQLite / PostgreSQL 两档通用）
echo ------------------------------------------------------------
echo   查看：      scripts\GM.cmd list
echo   等级：      scripts\GM.cmd set --name 角色名 --level 50        （加 --preview 只预览）
echo   点券/金币：  scripts\GM.cmd set --name 角色名 --cera +10000 --gold +5000000
echo   发物品：    scripts\GM.cmd set --name 角色名 --item 3037x10,20002
echo   操作记录：  scripts\GM.cmd history
echo ------------------------------------------------------------
echo   * 引擎按 runtime\storage\local.json 的 driver 自动判定（与服务端同一条规则）：
echo     SQLite 档直接读库文件；PostgreSQL 档需要库在监听，否则会提示先启动服务端。
echo   * 读取走 dfo-tool accountlist（只读、引擎中立）；写操作走 cmd/admin 与
echo     dfo-tool setlevel 的单事务 + 幂等键 + 审计路径；重复执行同一个 grant-id 只发一次。
echo   * 首次执行会准备 PVF 目录，约 30~60 秒（改等级只准备 progression 一个域，几秒）。
echo   * 角色在线时背包/余额仍用内存里的旧值，重选角色即可看到新值。
echo   * 改等级按 PVF 的累计经验阈值写 experience = Thresholds[level-2]（该等级起点），
echo     同步保证不会触发 level exceeds cumulative experience；不改技能点。
echo ============================================================
pause
exit /b 0

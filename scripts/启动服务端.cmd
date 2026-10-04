@echo off
chcp 65001 >nul
cd /d "%~dp0.."
title DFO 115us Server Gateway
rem 默认使用 configs/pvf-default.json；--repair-profile 可选其它配置，--json-mode 显式回退。
echo Starting DFO 115us Local Server (PVF Direct + Storage + Game Gateway)...
set DFO_SHOP_OPEN_ALL=1
set DFO_MAX_ITEM_PERIOD=1
set DFO_QUEST_VISIBLE_NPC_RELAX=1
set DFO_QUEST_NPC_DISTANCE_MULTIPLIER=5
rem 装备库「制作/变换」(CMD2259) 的应答会让客户端打开一个制作窗口：
rem   默认 1 = 窗口 3937；若点「制作/变换」没进制作界面，把下面这行改成 0 再启动，试窗口 2145。
rem set DFO_EQUIPMENT_CRAFT_WINDOW=0
rem 「装备生成」的执行时机：confirm（默认，同一次操作的第二次请求）/ first（第一次就执行，慎用）/ never。
rem 若点了生成没有任何反应、日志也只有一条 2259，可以试着放开下面这行改成 first 再启动。
rem set DFO_EQUIPMENT_CRAFT_EXECUTE_ON=first
rem 生成应答的子分支字节（payload[5]）：**默认 1** = 只落成功标志、不动窗口状态；
rem   设 0 = 强制 setState 到状态 3 —— 那个状态客户端自己不会进也没有出口，
rem   进去后「切换材料」按钮会失灵（实机 2026-09-29 14:36）。一般不用动。
rem set DFO_EQUIPMENT_CRAFT_GENERATE_VARIANT=0

rem Prefer the Go launcher: the session orchestration is Go, so this path needs no
rem Python runtime. DFO_ROOT is explicit because the launcher sits beside the repository
rem and would otherwise infer the wrong tree.
set "DFO_ROOT=%~dp0."
rem tools\ was moved out of the repository, so expose the moved Go toolchain on PATH for
rem the source build (serverbuild looks at tools\go first, then PATH).
set "PATH=%~dp0..\..\tools\go\bin;%PATH%"
if exist "..\115us-dfolauncher\bin\dfolauncher-cli.exe" (
    "..\115us-dfolauncher\bin\dfolauncher-cli.exe" --launch --server-only %*
) else if exist "tools\python\python.exe" (
    "tools\python\python.exe" "server\work\dfo-lan\scripts\launch_local.py" --server-only %*
) else (
    python "server\work\dfo-lan\scripts\launch_local.py" --server-only %*
)
if errorlevel 1 (
    echo.
    echo Server startup encountered an error.
    pause
    exit /b 1
)
echo.
pause

@echo off
chcp 65001 >nul
cd /d "%~dp0.."
title DFO 115us Game Launcher (Arad Odyssey Mode)
rem Same in-repository Go chain as the SQLite/story entries; PS handles UAC.
rem DFO_ADVENTURE_ELITE is inherited; exactly 1 enables the eligibility candidate.
set DFO_SHOP_OPEN_ALL=1
set DFO_ODYSSEY_MODE=1
set DFO_CONTRACT_PURCHASE_CRASH_FIX=1
set DFO_MAX_ITEM_PERIOD=1
set DFO_QUEST_VISIBLE_NPC_RELAX=1
set DFO_ADVENTURE_ELITE=1
powershell -NoProfile -ExecutionPolicy Bypass -File "scripts\storage-route.ps1" game-current %*
set "RC=%ERRORLEVEL%"
if not "%RC%"=="0" (
    echo.
    echo Game launch failed. Please inspect logs.
    pause
)
exit /b %RC%

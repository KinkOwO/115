@echo off
chcp 65001 >nul
cd /d "%~dp0"
title DFO 115us GM Dashboard
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0dashboard\start_gm_dashboard.ps1"

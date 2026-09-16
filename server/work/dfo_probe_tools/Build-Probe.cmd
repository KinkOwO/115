@echo off
setlocal
cd /d "%~dp0"
where cl >nul 2>nul
if errorlevel 1 (
  echo Run this in an x64 Visual Studio Developer Command Prompt.
  exit /b 1
)
cl /nologo /std:c++17 /EHsc /W4 /O2 /MT /utf-8 probe.cpp /Fe:probe-rebuilt.exe /link /SUBSYSTEM:CONSOLE

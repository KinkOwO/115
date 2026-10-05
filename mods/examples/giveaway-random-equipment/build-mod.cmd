@echo off
rem ASCII-only launcher for build-mod.py. Keep this file pure ASCII + CRLF:
rem cmd decodes batch text with the console code page, so non-ASCII (even in
rem rem lines) would be executed as garbage. All Chinese text comes from the .py.
setlocal
chcp 65001 >nul

set "PY=%~dp0build-mod.py"

set "PORT=%~dp0..\..\tools\python\python.exe"
if exist "%PORT%" (
  "%PORT%" -X utf8 "%PY%" %*
  exit /b %ERRORLEVEL%
)

set "PORT=%~dp0..\..\..\tools\python\python.exe"
if exist "%PORT%" (
  "%PORT%" -X utf8 "%PY%" %*
  exit /b %ERRORLEVEL%
)

set "PORT=%~dp0..\..\..\..\tools\python\python.exe"
if exist "%PORT%" (
  "%PORT%" -X utf8 "%PY%" %*
  exit /b %ERRORLEVEL%
)

where python >nul 2>nul
if %ERRORLEVEL%==0 (
  python -X utf8 "%PY%" %*
  exit /b %ERRORLEVEL%
)

where py >nul 2>nul
if %ERRORLEVEL%==0 (
  py -3 -X utf8 "%PY%" %*
  exit /b %ERRORLEVEL%
)

echo [ERROR] Python 3 not found.
echo   Install Python 3, or run: python build-mod.py
exit /b 1

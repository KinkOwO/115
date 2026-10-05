@echo off
chcp 65001 >nul
setlocal
cd /d "%~dp0"

rem Python is a GM-tool-only dependency (the game launch chain does not need it).
rem Resolution order: in-package GM dir -> GM-only copy next to tools\ -> python on PATH.
set "PY=%~dp0python\python.exe"
if not exist "%PY%" set "PY=%~dp0..\..\gm-tool\python\python.exe"
if exist "%PY%" goto :run
set "PY=python"
where python >nul 2>nul
if errorlevel 1 goto :no_python

:run
"%PY%" "%~dp0scripts\gmweb.py" %*
if errorlevel 1 (
    echo GM web startup failed. Read the error above.
    pause
    exit /b 1
)
exit /b 0

:no_python
echo [ERROR] Python interpreter not found.
echo         Python is a GM-tool-only dependency; the game launch chain does not need it.
echo         Looked for, in order:
echo           1) %~dp0python\python.exe
echo           2) %~dp0..\..\gm-tool\python\python.exe   (GM-only copy next to tools\)
echo           3) python on PATH
echo         To restore: unzip the portable Python 3.11 (capstone / cryptography / pefile / frida)
echo         into the "gm-tool\python" folder that sits next to the package's "tools" folder,
echo         or put python on PATH.
pause
exit /b 1
@echo off
rem Restore DFO.exe from the most recent backup made by patch_font_scale.py.
rem The game MUST be closed before running this.
setlocal
set PY=C:\Game\dof\115us\115\tools\python\python.exe
set TOOL=%~dp0patch_font_scale.py
if not exist "%PY%" set PY=python
"%PY%" "%TOOL%" restore
echo.
pause
endlocal

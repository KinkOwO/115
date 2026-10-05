@echo off
rem Restore the stock UI font scale (K 1.1 -> 1.0) in DFO.exe.
rem The game MUST be closed before running this.
setlocal
rem Python is a GM-tool-only dependency and no longer lives in tools\ (the game launch
rem chain does not need it): GM-only copy outside the package first, then python on PATH.
set "PY=%~dp0..\..\..\gm-tool\python\python.exe"
if exist "%PY%" goto :py_ready
set "PY=python"
where python >nul 2>nul
if errorlevel 1 goto :no_python
:py_ready
set TOOL=%~dp0patch_font_scale.py
"%PY%" "%TOOL%" status
echo.
echo This will set the font scale multiplier to 1.0 (stock size).
pause
"%PY%" "%TOOL%" set-scale 1.0
echo.
echo Done. Start the game and check the UI text.
echo To undo, run: restore-fontsize.cmd
pause
endlocal
exit /b 0

:no_python
endlocal
echo [ERROR] Python interpreter not found.
echo         Python is a GM-tool-only dependency; the game launch chain does not need it.
echo         Looked for, in order:
echo           1) a python.exe in the "gm-tool\python" folder next to the package's "tools" folder
echo           2) python on PATH
echo         To restore: unzip the portable Python 3.11 (capstone / cryptography / pefile / frida)
echo         into that gm-tool\python folder, or put python on PATH.
pause
exit /b 1

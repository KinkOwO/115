@echo off
rem Restore the stock UI font scale (K 1.1 -> 1.0) in DFO.exe.
rem The game MUST be closed before running this.
setlocal
set PY=C:\Game\dof\115us\115\tools\python\python.exe
set TOOL=%~dp0patch_font_scale.py
if not exist "%PY%" set PY=python
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

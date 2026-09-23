@echo off
REM X7 probe: resolve the class behind the 144-byte NOTI2657 phase block.
REM Runs headlessly against the working copy of the IDB; the authoritative
REM client/DFO.exe.i64 is never opened.
REM
REM Output: D:\115us\analysis\dumps\phase-x7\ + D:\115us-backup\ida-phase-x7.log
setlocal
set "IDAT=D:\tools\ida94\idat.exe"
set "WORK=D:\115us-backup\ida-work\DFO.exe.i64"
set "SCRIPT=D:\115us\analysis\dumps\ida_phase_x7.py"
set "LOG=D:\115us-backup\ida-phase-x7.log"
set "PYTHONPATH="
set "PYTHONHOME="
if not exist "%IDAT%" echo missing %IDAT% & exit /b 1
if not exist "%WORK%" echo missing %WORK% & exit /b 1
"%IDAT%" -A -L"%LOG%" -S"%SCRIPT%" "%WORK%"
echo idat exit=%ERRORLEVEL%
echo log: %LOG%
echo out: D:\115us\analysis\dumps\phase-x7\summary.json

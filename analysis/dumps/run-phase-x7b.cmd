@echo off
REM X7 probe pass 2: scope the NOTI2657 dispatcher (callers + the global it uses).
REM
REM IDA unpacks the IDB into DFO.exe.id0/.id1/.id2/.nam/.til next to it. A run
REM that is interrupted leaves those behind, and the next start then warns
REM "IDA did not close properly" and may come up with an unusable database --
REM so this script sweeps them both before and after the run.
setlocal
set "IDAT=D:\tools\ida94\idat.exe"
set "WORK=D:\115us-backup\ida-work\DFO.exe.i64"
set "BASE=D:\115us-backup\ida-work\DFO.exe"
set "SCRIPT=D:\115us\analysis\dumps\ida_phase_x7b.py"
set "LOG=D:\115us-backup\ida-phase-x7b.log"
set "PYTHONPATH="
set "PYTHONHOME="
if not exist "%IDAT%" echo missing %IDAT% & exit /b 1
if not exist "%WORK%" echo missing %WORK% & exit /b 1
for %%E in (id0 id1 id2 nam til) do if exist "%BASE%.%%E" del /q "%BASE%.%%E"
"%IDAT%" -A -L"%LOG%" -S"%SCRIPT%" "%WORK%"
set RC=%ERRORLEVEL%
for %%E in (id0 id1 id2 nam til) do if exist "%BASE%.%%E" del /q "%BASE%.%%E"
echo idat exit=%RC%
echo out: D:\115us\analysis\dumps\phase-x7b\summary.json

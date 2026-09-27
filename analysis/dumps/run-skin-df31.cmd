@echo off
REM df31: does any function read the holder's per-slot damage-font maps (a1+120 /
REM a1+136)? df30 proved the renderer branches on the pool record's own id and
REM that NOTI1546 only ever writes slot 0. If a slot-indexed reader exists, only
REM NOTI1672 (entry index = slot) can change that layer.
REM
REM IDA unpacks the IDB into DFO.exe.id0/.id1/.id2/.nam/.til next to it, which in
REM this workspace would sit beside the real DFO.exe payload; sweep them before
REM and after so an interrupted run never leaves a half-written database.
setlocal
set "IDAT=C:\Program Files\IDA Professional 9.3\idat.exe"
set "WORK=D:\115us\analysis\ida-work\DFO.exe.i64"
set "BASE=D:\115us\analysis\ida-work\DFO.exe"
set "SCRIPT=D:\115us\analysis\dumps\ida_skin_df31.py"
set "LOG=D:\115us\analysis\ida-work\df31.log"
set "PYTHONPATH="
set "PYTHONHOME="
if not exist "%IDAT%" echo missing %IDAT% & exit /b 1
if not exist "%WORK%" echo missing %WORK% & exit /b 1
for %%E in (id0 id1 id2 nam til) do if exist "%BASE%.%%E" del /q "%BASE%.%%E"
"%IDAT%" -A -L"%LOG%" -S"%SCRIPT%" "%WORK%"
set RC=%ERRORLEVEL%
for %%E in (id0 id1 id2 nam til) do if exist "%BASE%.%%E" del /q "%BASE%.%%E"
echo idat exit=%RC%
echo log: %LOG%

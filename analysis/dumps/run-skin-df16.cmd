@echo off
REM df16: pin down which cargo page index is the damage-font tab.
REM
REM IDA unpacks the IDB into DFO.exe.id0/.id1/.id2/.nam/.til next to it, which in
REM this workspace would sit beside the real DFO.exe payload; sweep them before
REM and after so an interrupted run never leaves a half-written database.
setlocal
set "IDAT=C:\Program Files\IDA Professional 9.3\idat.exe"
set "WORK=D:\115us\analysis\ida-work\DFO.exe.i64"
set "BASE=D:\115us\analysis\ida-work\DFO.exe"
set "SCRIPT=D:\115us\analysis\dumps\ida_skin_df16.py"
set "LOG=D:\115us\analysis\ida-work\df16.log"
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

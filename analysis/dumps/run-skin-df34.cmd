@echo off
REM df34: how does the client fill the static skin registry record's family class (+8) and
REM subtype (+12)? The PVF [type]/[sub type] labels are strings that do not occur in DFO.exe,
REM yet every panel grid reader filters rec+8 == its owned page, so the int mapping is the
REM missing link for the party-frame (page 0) and skill-cutscene (page 1) families.
REM IDA unpacks the IDB into DFO.exe.id0/.id1/.id2/.nam/.til next to it, which would
REM sit beside the real DFO.exe payload; sweep before and after so an interrupted
REM run never leaves a half-written database.
setlocal
set "IDAT=C:\Program Files\IDA Professional 9.3\idat.exe"
set "WORK=D:\115us\analysis\ida-work\DFO.exe.i64"
set "BASE=D:\115us\analysis\ida-work\DFO.exe"
set "SCRIPT=D:\115us\analysis\dumps\ida_skin_df34.py"
set "LOG=D:\115us\analysis\ida-work\df34.log"
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

@echo off
REM df35: what fills the static skin registry record's +8 (family class) and +12 (subtype)?
REM df34 closed: every reference to qword_14E683BF8 inside the skin-cargo cluster is a READ
REM through sub_140283D60; the three non-read sites are sub_146D7BDC0 / sub_146D7C710 /
REM sub_146D7CD10, which df34's candidate cap never reached.
REM IDA unpacks the IDB into DFO.exe.id0/.id1/.id2/.nam/.til next to it, which would
REM sit beside the real DFO.exe payload; sweep before and after so an interrupted
REM run never leaves a half-written database.
setlocal
set "IDAT=C:\Program Files\IDA Professional 9.3\idat.exe"
set "WORK=D:\115us\analysis\ida-work\DFO.exe.i64"
set "BASE=D:\115us\analysis\ida-work\DFO.exe"
set "SCRIPT=D:\115us\analysis\dumps\ida_skin_df35.py"
set "LOG=D:\115us\analysis\ida-work\df35.log"
set "PYTHONPATH="
set "PYTHONHOME="
if not exist "%IDAT%" echo missing %IDAT% & exit /b 1
if not exist "%WORK%" echo missing %WORK% & exit /b 1
if not exist "%SCRIPT%" echo missing %SCRIPT% & exit /b 1
for %%E in (id0 id1 id2 nam til) do if exist "%BASE%.%%E" del /q "%BASE%.%%E"
"%IDAT%" -A -L"%LOG%" -S"%SCRIPT%" "%WORK%"
set RC=%ERRORLEVEL%
for %%E in (id0 id1 id2 nam til) do if exist "%BASE%.%%E" del /q "%BASE%.%%E"
echo idat exit=%RC%
echo log: %LOG%

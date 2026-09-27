@echo off
REM df37: is the dungeon-time 二次觉醒 cutscene id read live from mgr+1152 - the list
REM        NOTI1546 category 1 fills from its SECOND list, which this server always sends empty?
REM        Anchors: sub_1444EBAD0 (only reader of +1152), its callers, sub_1444EBB90 /
REM        sub_1444EA9D0, the twelve cluster writers of +1128 / +1152 / +1176, and the two
REM        gates sub_145CD39F0 / sub_145CF7C40 used by sub_145D451D0.
REM
REM IDA unpacks the IDB into DFO.exe.id0/.id1/.id2/.nam/.til next to it, which in
REM this workspace would sit beside the real DFO.exe payload; sweep them before
REM and after so an interrupted run never leaves a half-written database.
setlocal
set "IDAT=C:\Program Files\IDA Professional 9.3\idat.exe"
set "WORK=D:\115us\analysis\ida-work\DFO.exe.i64"
set "BASE=D:\115us\analysis\ida-work\DFO.exe"
set "SCRIPT=D:\115us\analysis\dumps\ida_skin_df37.py"
set "LOG=D:\115us\analysis\ida-work\df37.log"
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

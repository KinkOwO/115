@echo off
REM Decompile the legion/apocalypse NOTI consumers (X9 / X11 structures).
setlocal
set "IDAT=D:\tools\ida94\idat.exe"
set "WORK=D:\115us-backup\ida-work\DFO.exe.i64"
set "BASE=D:\115us-backup\ida-work\DFO.exe"
set "SCRIPT=D:\115us\analysis\dumps\ida_legion_noti_consumers.py"
set "LOG=D:\115us-backup\ida-legion-noti.log"
set "PYTHONPATH="
set "PYTHONHOME="
if not exist "%IDAT%" echo missing %IDAT% & exit /b 1
if not exist "%WORK%" echo missing %WORK% & exit /b 1
for %%E in (id0 id1 id2 nam til) do if exist "%BASE%.%%E" del /q "%BASE%.%%E"
"%IDAT%" -A -L"%LOG%" -S"%SCRIPT%" "%WORK%"
set RC=%ERRORLEVEL%
for %%E in (id0 id1 id2 nam til) do if exist "%BASE%.%%E" del /q "%BASE%.%%E"
echo idat exit=%RC%
echo out: D:\115us\analysis\dumps\legion-noti-consumers\summary.json

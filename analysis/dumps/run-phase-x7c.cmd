@echo off
REM X7 probe pass 3: xrefs only (no decompilation), so it returns in seconds.
setlocal
set "IDAT=D:\tools\ida94\idat.exe"
set "WORK=D:\115us-backup\ida-work\DFO.exe.i64"
set "BASE=D:\115us-backup\ida-work\DFO.exe"
set "SCRIPT=D:\115us\analysis\dumps\ida_phase_x7c.py"
set "LOG=D:\115us-backup\ida-phase-x7c.log"
set "PYTHONPATH="
set "PYTHONHOME="
if not exist "%IDAT%" echo missing %IDAT% & exit /b 1
if not exist "%WORK%" echo missing %WORK% & exit /b 1
for %%E in (id0 id1 id2 nam til) do if exist "%BASE%.%%E" del /q "%BASE%.%%E"
"%IDAT%" -A -L"%LOG%" -S"%SCRIPT%" "%WORK%"
set RC=%ERRORLEVEL%
for %%E in (id0 id1 id2 nam til) do if exist "%BASE%.%%E" del /q "%BASE%.%%E"
echo idat exit=%RC%

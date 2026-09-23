@echo off
REM Decompile the legion entry trigger points (sub_142510A50 / sub_142511D10).
setlocal
set "IDAT=D:\tools\ida94\idat.exe"
set "WORK=D:\115us-backup\ida-work\DFO.exe.i64"
set "BASE=D:\115us-backup\ida-work\DFO.exe"
set "SCRIPT=D:\115us\analysis\dumps\ida_legion_entry_gate.py"
set "LOG=D:\115us-backup\ida-legion-entry.log"
set "PYTHONPATH="
set "PYTHONHOME="
if not exist "%IDAT%" echo missing %IDAT% & exit /b 1
if not exist "%WORK%" echo missing %WORK% & exit /b 1
for %%E in (id0 id1 id2 nam til) do if exist "%BASE%.%%E" del /q "%BASE%.%%E"
"%IDAT%" -A -L"%LOG%" -S"%SCRIPT%" "%WORK%"
set RC=%ERRORLEVEL%
for %%E in (id0 id1 id2 nam til) do if exist "%BASE%.%%E" del /q "%BASE%.%%E"
echo idat exit=%RC%
echo out: D:\115us\analysis\dumps\legion-entry-gate\summary.json

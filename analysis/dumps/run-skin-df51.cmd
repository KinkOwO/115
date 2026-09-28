@echo off
REM df51: name the writer of the skin row's star checkbox state (defect: clicking the star does not light it)
setlocal
set "IDAT=C:\Program Files\IDA Professional 9.3\idat.exe"
set "WORK=D:\115us\analysis\ida-work\DFO.exe.i64"
set "BASE=D:\115us\analysis\ida-work\DFO.exe"
set "SCRIPT=D:\115us\analysis\dumps\ida_skin_df51.py"
set "LOG=D:\115us\analysis\ida-work\df51.log"
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

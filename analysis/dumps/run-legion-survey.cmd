@echo off
REM Runs the legion/apocalypse IDA survey headlessly against a working copy of
REM the IDB. The authoritative client/DFO.exe.i64 is never touched: IDA opens
REM D:\115us-backup\ida-work\DFO.exe.i64 instead.
REM
REM Prerequisites (already applied on this machine):
REM   - IDA Professional 9.4 installed at D:\tools\ida94 (unattended install)
REM   - IDAPython bound to the project Python 3.11.9 via idapyswitch -s
REM   - PYTHONHOME/PYTHONPATH cleared, otherwise IDAPython fails to start
REM   - a valid ida.hexlic (IDA refuses to run without a license)
REM
REM Output: D:\115us\analysis\dumps\legion-survey\ + D:\115us-backup\ida-survey.log
setlocal
set "IDAT=D:\tools\ida94\idat.exe"
set "WORK=D:\115us-backup\ida-work\DFO.exe.i64"
set "SCRIPT=D:\115us\analysis\dumps\ida_legion_survey.py"
set "LOG=D:\115us-backup\ida-survey.log"
set "PYTHONPATH="
set "PYTHONHOME="
if not exist "%IDAT%" echo missing %IDAT% & exit /b 1
if not exist "%WORK%" echo missing %WORK% & exit /b 1
"%IDAT%" -A -L"%LOG%" -S"%SCRIPT%" "%WORK%"
echo idat exit=%ERRORLEVEL%
echo log: %LOG%
echo survey: D:\115us\analysis\dumps\legion-survey\survey.json

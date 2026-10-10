@echo off
REM Build and run the offline self-test for the hp-caller-probe ring/tally logic.
REM ASCII only + CRLF. Touches nothing outside client-patchs\tests\.
setlocal
set "VCVARS=C:\Program Files\Microsoft Visual Studio\2022\Enterprise\VC\Auxiliary\Build\vcvars64.bat"
if not exist "%VCVARS%" (
  echo [FAIL] vcvars64.bat not found: %VCVARS%
  exit /b 1
)
call "%VCVARS%" >nul
pushd "%~dp0"
cl /nologo /O2 /MT /W4 /GS /utf-8 /TP hp-probe-selftest.c /Fe:hp-probe-selftest.exe ^
   /link /NOIMPLIB kernel32.lib
set RC=%ERRORLEVEL%
if not "%RC%"=="0" (
  echo BUILD_EXIT=%RC%
  popd
  exit /b %RC%
)
echo BUILD_EXIT=0
hp-probe-selftest.exe
set RC=%ERRORLEVEL%
popd
echo RUN_EXIT=%RC%
exit /b %RC%

@echo off
REM Build and run the pure-logic self-test (no game, no client memory). ASCII only.
setlocal
set "VCVARS=C:\Program Files\Microsoft Visual Studio\2022\Enterprise\VC\Auxiliary\Build\vcvars64.bat"
if not exist "%VCVARS%" (
  echo [FAIL] vcvars64.bat not found: %VCVARS%
  exit /b 1
)
call "%VCVARS%" >nul
set "SRC=%~dp0..\src"
set "OUT=%~dp0..\dist\selftest"
if not exist "%OUT%" mkdir "%OUT%"
pushd "%OUT%"
cl /nologo /O2 /MT /W3 /utf-8 /I"%SRC%" "%~dp0difficulty-rules.selftest.c" /Fe:difficulty-selftest.exe
if errorlevel 1 (
  echo SELFTEST_BUILD_EXIT=1
  popd
  exit /b 1
)
popd
"%OUT%\difficulty-selftest.exe"
echo SELFTEST_EXIT=%ERRORLEVEL%

@echo off
REM Build and run the offline self-test that EXECUTES the hp-caller-probe code
REM cave (assembled from hp-caller-probe\src\probe-hook.s) and proves it leaves
REM every register the x64 ABI requires intact.  ASCII only + CRLF.
REM Touches nothing outside client-patchs\tests\.
setlocal
set "VCVARS=C:\Program Files\Microsoft Visual Studio\2022\Enterprise\VC\Auxiliary\Build\vcvars64.bat"
if not exist "%VCVARS%" (
  echo [FAIL] vcvars64.bat not found: %VCVARS%
  exit /b 1
)
call "%VCVARS%" >nul
pushd "%~dp0"

if not exist "..\hp-caller-probe\src\probe-hook-image.h" (
  echo [FAIL] src\probe-hook-image.h is missing - run hp-caller-probe\build-dll.cmd first.
  popd
  exit /b 1
)

echo [1/2] assembling the harness entry/landing pad ...
ml64 /nologo /c /Fo cave_test_entry.obj cave_test_entry.asm
if errorlevel 1 (
  popd
  exit /b 1
)

echo [2/2] compiling and running the cave self-test ...
cl /nologo /O2 /MT /W4 /GS /utf-8 /TP /c cave-selftest.c
if errorlevel 1 (
  popd
  exit /b 1
)
REM /OPT:NOICF is required: the embedded cave image is mostly zeros and the
REM linker's identical-COMDAT folding would silently alias those runs with
REM other zero data, corrupting the image.
link /nologo /OPT:NOICF cave-selftest.obj cave_test_entry.obj /OUT:cave-selftest.exe kernel32.lib
set RC=%ERRORLEVEL%
if not "%RC%"=="0" (
  echo BUILD_EXIT=%RC%
  popd
  exit /b %RC%
)
echo BUILD_EXIT=0
cave-selftest.exe
set RC=%ERRORLEVEL%
popd
echo RUN_EXIT=%RC%
exit /b %RC%

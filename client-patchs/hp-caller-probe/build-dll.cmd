@echo off
REM Build the HP-caller runtime probe plugin DLL (x64, static CRT).
REM ASCII only + CRLF. Output: dist\HpCallerProbe.dll
REM
REM Steps:
REM   1. tools\cavegen.py assembles src\probe-hook.s with ml64, disassembles the
REM      result and regenerates src\probe-hook-image.h (it refuses to write the
REM      header if the trampoline layout changed).
REM   2. cl builds the plugin DLL from src\hp-caller-probe.c (which embeds that
REM      byte image).  /TP is required: the source uses extern "C" so the two
REM      plugin exports keep their undecorated C names.
REM      /OPT:NOICF is required: the embedded cave image is mostly zeros and the
REM      linker's identical-COMDAT folding would alias those runs with other
REM      zero data (observed in the offline harness as a zeroed jump slot).
REM      /Brepro makes the output reproducible (no per-build timestamp).
setlocal
set "VCVARS=C:\Program Files\Microsoft Visual Studio\2022\Enterprise\VC\Auxiliary\Build\vcvars64.bat"
if not exist "%VCVARS%" (
  echo [FAIL] vcvars64.bat not found: %VCVARS%
  exit /b 1
)
set "ROOT=%~dp0"
set "SRC=%ROOT%src"
set "OUT=%ROOT%dist"
if not exist "%OUT%" mkdir "%OUT%"

echo [1/2] assembling and verifying the code cave ...
python "%ROOT%tools\cavegen.py"
if errorlevel 1 (
  echo [FAIL] cavegen.py failed - the trampoline layout is not what the plugin expects.
  exit /b 1
)

echo [2/2] compiling the plugin DLL ...
call "%VCVARS%" >nul
pushd "%OUT%"
cl /nologo /LD /O2 /MT /W4 /GS /utf-8 /TP "%SRC%\hp-caller-probe.c" ^
   /Fe:HpCallerProbe.dll ^
   /link /NOIMPLIB /NOEXP /OPT:NOICF /Brepro /EXPORT:ModStart /EXPORT:ModName ^
   kernel32.lib user32.lib
set RC=%ERRORLEVEL%
popd
echo BUILD_EXIT=%RC%
if exist "%OUT%\HpCallerProbe.dll" dir "%OUT%\HpCallerProbe.dll"
exit /b %RC%

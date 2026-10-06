@echo off
REM Build + run the probe smoke test. ASCII only. Not part of the game mod.
setlocal
set "VCVARS=C:\Program Files\Microsoft Visual Studio\2022\Enterprise\VC\Auxiliary\Build\vcvars64.bat"
if not exist "%VCVARS%" exit /b 1
call "%VCVARS%" >nul
set "SRC=%~dp0src"
set "OUT=%~dp0dist"
pushd "%OUT%"
cl /nologo /O2 /MT /W4 /utf-8 "%SRC%\test-host.c" /Fe:test-host.exe /link /NOIMPLIB user32.lib
set RC=%ERRORLEVEL%
popd
echo BUILD_EXIT=%RC%
if not "%RC%"=="0" exit /b %RC%
del "%OUT%\chinese-input-probe.log" >nul 2>&1
"%OUT%\test-host.exe" "%OUT%\ChineseLocalization.dll"
echo HOST_EXIT=%ERRORLEVEL%

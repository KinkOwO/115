@echo off
REM Build the client mod host DLL (x64, MSVC). ASCII only.
REM Output: ..\dist\ChineseLocalization.dll
setlocal
set "VCVARS=C:\Program Files\Microsoft Visual Studio\2022\Enterprise\VC\Auxiliary\Build\vcvars64.bat"
if not exist "%VCVARS%" (
  echo [FAIL] vcvars64.bat not found: %VCVARS%
  exit /b 1
)
call "%VCVARS%" >nul
set "SRC=%~dp0src"
set "OUT=%~dp0dist"
if not exist "%OUT%" mkdir "%OUT%"
pushd "%OUT%"
cl /nologo /LD /O2 /MT /W4 /GS /utf-8 "%SRC%\client-host.c" ^
   /Fe:ChineseLocalization.dll ^
   /link /NOIMPLIB /DEF:"%SRC%\ChineseLocalization.def" ^
   user32.lib
set RC=%ERRORLEVEL%
popd
echo BUILD_EXIT=%RC%
if exist "%OUT%\ChineseLocalization.dll" dir "%OUT%\ChineseLocalization.dll"

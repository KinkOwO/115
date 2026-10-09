@echo off
REM Build the DifficultyRules host plugin DLL (x64, MSVC, static CRT).
REM ASCII only (AGENTS 0.4.2). Output: dist\DifficultyRules.dll
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
cl /nologo /LD /O2 /MT /W3 /utf-8 "%SRC%\difficulty-rules.c" ^
   /Fe:DifficultyRules.dll ^
   /link /INCREMENTAL:NO /NOIMPLIB /DEF:"%SRC%\DifficultyRules.def"
set RC=%ERRORLEVEL%
popd
echo BUILD_EXIT=%RC%
if exist "%OUT%\DifficultyRules.dll" dir "%OUT%\DifficultyRules.dll"

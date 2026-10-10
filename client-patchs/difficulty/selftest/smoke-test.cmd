@echo off
REM Plugin-ABI smoke test: load DifficultyRules.dll the way the host does.
REM ASCII only. Does NOT start the game.
setlocal
set "VCVARS=C:\Program Files\Microsoft Visual Studio\2022\Enterprise\VC\Auxiliary\Build\vcvars64.bat"
if not exist "%VCVARS%" (
  echo [FAIL] vcvars64.bat not found: %VCVARS%
  exit /b 1
)
call "%VCVARS%" >nul
set "HERE=%~dp0"
set "DLL=%~dp0..\dist\DifficultyRules.dll"
set "OUT=%~dp0..\dist\smoke"
if not exist "%DLL%" (
  echo [FAIL] build the DLL first: build-dll.cmd
  exit /b 1
)
if not exist "%OUT%" mkdir "%OUT%"

pushd "%OUT%"
cl /nologo /O2 /MT /W3 /utf-8 "%HERE%host-smoke.c" /Fe:host-smoke.exe
if errorlevel 1 (
  echo SMOKE_BUILD_EXIT=1
  popd
  exit /b 1
)
popd

rmdir /s /q "%OUT%\fake" >nul 2>&1
mkdir "%OUT%\fake\.115us-mods" >nul
copy /y "%DLL%" "%OUT%\fake\.115us-mods\DifficultyRules.dll" >nul

echo --- loading the plugin the way the host does ---
"%OUT%\host-smoke.exe" "%OUT%\fake\.115us-mods\DifficultyRules.dll"
echo SMOKE_EXIT=%ERRORLEVEL%

echo --- plugin log ---
if exist "%OUT%\fake\.115us-mods\difficulty-rules.log" (
  powershell -NoProfile -Command "Get-Content -Encoding UTF8 '%OUT%\fake\.115us-mods\difficulty-rules.log'"
) else (
  echo [FAIL] no difficulty-rules.log written by the plugin
)
echo --- plugin status file ---
if exist "%OUT%\fake\.115us-mods\difficulty-rules.status.json" (
  type "%OUT%\fake\.115us-mods\difficulty-rules.status.json"
) else (
  echo [warn] no difficulty-rules.status.json (only written when the plugin runs its worker)
)

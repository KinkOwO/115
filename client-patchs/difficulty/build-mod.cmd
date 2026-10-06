@echo off
REM Package client-patchs/difficulty into a schema-2 mod zip. ASCII only.
REM Requires dist\DifficultyRules.dll (run build-dll.cmd first).
setlocal
set "ROOT=%~dp0"
set "PY=python"
if exist "C:\Users\Ricar\anaconda3\python.exe" set "PY=C:\Users\Ricar\anaconda3\python.exe"
if not exist "%ROOT%dist\DifficultyRules.dll" (
  echo [FAIL] dist\DifficultyRules.dll missing - run build-dll.cmd first
  exit /b 1
)
"%PY%" "%ROOT%build-mod.py"
echo BUILD_MOD_EXIT=%ERRORLEVEL%

@echo off
REM Gate self-test: replicate the client's IME gate and prove the plugin flips it.
REM Usage: build-gate.cmd <absolute path to ChineseLocalization.dll>
REM (Chinese help text lives in gate_test.c, not here: .cmd must stay ASCII.)
setlocal
call "C:\Program Files\Microsoft Visual Studio\2022\Enterprise\VC\Auxiliary\Build\vcvars64.bat" >nul
cd /d "%~dp0"
cl /nologo /O2 /MT /W4 /utf-8 gate_test.c /Fe:gate_test.exe /link /NOIMPLIB user32.lib
echo BUILD_EXIT=%ERRORLEVEL%
if not "%ERRORLEVEL%"=="0" exit /b %ERRORLEVEL%
if "%~1"=="" (
  echo Usage: build-gate.cmd ^<path to ChineseLocalization.dll^>
  exit /b 1
)
gate_test.exe "%~1"
echo RUN_EXIT=%ERRORLEVEL%

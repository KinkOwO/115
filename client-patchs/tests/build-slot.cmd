@echo off
REM End-to-end test of the Themida import-slot fixup: build a fake client with a
REM real slot in the scanned RVA range, load the plugin, call through the slot.
REM Usage: build-slot.cmd <absolute path to ChineseLocalization.dll>
setlocal
call "C:\Program Files\Microsoft Visual Studio\2022\Enterprise\VC\Auxiliary\Build\vcvars64.bat" >nul
cd /d "%~dp0"
cl /nologo /O2 /MT /W4 /utf-8 slot_test.c /Fe:slot_test.exe /link /NOIMPLIB user32.lib
echo BUILD_EXIT=%ERRORLEVEL%
if not "%ERRORLEVEL%"=="0" exit /b %ERRORLEVEL%
if "%~1"=="" (
  echo Usage: build-slot.cmd ^<path to ChineseLocalization.dll^>
  exit /b 1
)
slot_test.exe "%~1"
echo RUN_EXIT=%ERRORLEVEL%

@echo off
setlocal
call "C:\Program Files\Microsoft Visual Studio\2022\Enterprise\VC\Auxiliary\Build\vcvars64.bat" >nul
cd /d "%~dp0"
cl /nologo /O2 /MT /W4 /utf-8 inline_test.c /Fe:inline_test.exe /link /NOIMPLIB
echo BUILD_EXIT=%ERRORLEVEL%
if not "%ERRORLEVEL%"=="0" exit /b %ERRORLEVEL%
inline_test.exe
echo RUN_EXIT=%ERRORLEVEL%

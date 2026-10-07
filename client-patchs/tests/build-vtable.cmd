@echo off
setlocal
call "C:\Program Files\Microsoft Visual Studio\2022\Enterprise\VC\Auxiliary\Build\vcvars64.bat" >nul
cd /d "C:\Game\dof\115us\115\.tmp\ime-recon\inline-test"
cl /nologo /O2 /MT /W4 /utf-8 vtable_test.c /Fe:vtable_test.exe /link /NOIMPLIB
echo BUILD_EXIT=%ERRORLEVEL%
if not "%ERRORLEVEL%"=="0" exit /b %ERRORLEVEL%
vtable_test.exe
echo RUN_EXIT=%ERRORLEVEL%

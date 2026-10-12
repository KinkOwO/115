@echo off
setlocal
if not defined VCVARS (
  for /f "usebackq delims=" %%I in (`"%ProgramFiles(x86)%\Microsoft Visual Studio\Installer\vswhere.exe" -latest -products * -requires Microsoft.VisualStudio.Component.VC.Tools.x86.x64 -property installationPath`) do set "VCVARS=%%I\VC\Auxiliary\Build\vcvars64.bat"
)
if not exist "%VCVARS%" exit /b 1
call "%VCVARS%" >nul
if defined SDK_INCLUDE set "INCLUDE=%SDK_INCLUDE%;%INCLUDE%"
if defined SDK_LIB set "LIB=%SDK_LIB%;%LIB%"
if errorlevel 1 exit /b %ERRORLEVEL%
if not exist "%~dp0..\apocalypse-third-card\dist" mkdir "%~dp0..\apocalypse-third-card\dist"
pushd "%~dp0..\apocalypse-third-card\dist"
cl /nologo /O2 /MT /W4 /WX /GS /utf-8 "%~dp0apocalypse-card-test.c" /Fe:apocalypse-card-test.exe
if errorlevel 1 (popd & exit /b 1)
apocalypse-card-test.exe
set RC=%ERRORLEVEL%
popd
exit /b %RC%

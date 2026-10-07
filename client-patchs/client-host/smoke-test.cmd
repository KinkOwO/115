@echo off
REM Smoke test for the client mod host + plugin chain. ASCII only.
REM Builds nothing: expects dist\ChineseLocalization.dll (host) and
REM ..\chinese-input\dist\{ChineseLocalization.dll,test-host.exe} to exist.
setlocal
set "HERE=%~dp0"
set "CH=%~dp0..\chinese-input\dist"
set "CASE=%~dp0dist\faketest"

if not exist "%HERE%dist\ChineseLocalization.dll" ( echo [FAIL] build the host first & exit /b 1 )
if not exist "%CH%\ChineseLocalization.dll"        ( echo [FAIL] build the probe first & exit /b 1 )
if not exist "%CH%\test-host.exe"                  ( echo [FAIL] build the test host first & exit /b 1 )

rmdir /s /q "%CASE%" >nul 2>&1
mkdir "%CASE%" >nul
mkdir "%CASE%\.115us-mods" >nul
copy /y "%HERE%dist\ChineseLocalization.dll" "%CASE%\ChineseLocalization.dll" >nul
copy /y "%CH%\ChineseLocalization.dll" "%CASE%\.115us-mods\ChineseInputProbe.dll" >nul
if exist "%CH%\..\chinese-input.ini" copy /y "%CH%\..\chinese-input.ini" "%CASE%\.115us-mods\chinese-input.ini" >nul

echo --- running host+plugin in a fake client dir ---
"%CH%\test-host.exe" "%CASE%\ChineseLocalization.dll"
echo HOST_EXIT=%ERRORLEVEL%

echo --- host log ---
if exist "%CASE%\client-host.log" ( type "%CASE%\client-host.log" ) else ( echo [FAIL] no client-host.log )
echo --- plugin log (first lines) ---
if exist "%CASE%\.115us-mods\chinese-input-probe.log" ( powershell -NoProfile -Command "Get-Content -Encoding UTF8 '%CASE%\.115us-mods\chinese-input-probe.log' -TotalCount 6" ) else ( echo [FAIL] no plugin log )

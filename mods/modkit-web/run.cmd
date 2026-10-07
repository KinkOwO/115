@echo off
REM modkit-web launcher.
REM   Preference: built exe next to this script -> Go on PATH -> bundled Go under ..\..\..\tools
REM   ASCII only on purpose (repo rule: .cmd must stay pure ASCII + CRLF).
setlocal
cd /d "%~dp0"

set "MODSDIR=%~dp0.."
rem Default args: no --open (the browser is NOT opened automatically).
rem Pass --open yourself when you want it.
set "DEFAULTARGS=--scan-depth 2"
if not "%~1"=="" set "DEFAULTARGS="

if exist "%~dp0modkit-web.exe" (
  echo [modkit-web] running built exe
  "%~dp0modkit-web.exe" --mods-dir "%MODSDIR%" %DEFAULTARGS% %*
  goto :eof
)

set "GOEXE="
for %%I in (go.exe) do if not defined GOEXE set "GOEXE=%%~$PATH:I"
if not defined GOEXE if exist "%~dp0..\..\..\tools\go\bin\go.exe" set "GOEXE=%~dp0..\..\..\tools\go\bin\go.exe"
if not defined GOEXE if exist "C:\Game\dof\115us\tools\go\bin\go.exe" set "GOEXE=C:\Game\dof\115us\tools\go\bin\go.exe"

if not defined GOEXE (
  echo [modkit-web] Go not found on PATH and not in the bundled tools folder.
  echo [modkit-web] Build once with the bundled Go, then re-run this script:
  echo   C:\Game\dof\115us\tools\go\bin\go.exe build -o modkit-web.exe .
  exit /b 1
)

echo [modkit-web] running from source with %GOEXE%
"%GOEXE%" run . --mods-dir "%MODSDIR%" %DEFAULTARGS% %*

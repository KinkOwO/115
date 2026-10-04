@echo off

chcp 65001 >nul

cd /d "%~dp0.."

title Check DFO 115us Environment

echo Checking DFO 115us dependencies (read-only; starts and stops nothing)...



if exist "server\work\dfo-lan\bin\dfolauncher.exe" (

    "server\work\dfo-lan\bin\dfolauncher.exe" check --root "%~dp0.." %*

    goto :done

)



set "GO_CHECK_RC=1"

if exist "tools\go\bin\go.exe" call :go_check

if "%GO_CHECK_RC%"=="0" goto :done

if exist "tools\go\bin\go.exe" echo Go launcher unavailable; falling back to Python.



if exist "tools\python\python.exe" (

    "tools\python\python.exe" "server\work\dfo-lan\scripts\launch_local.py" --check %*

) else (

    python "server\work\dfo-lan\scripts\launch_local.py" --check %*

)



:done

pause

exit /b 0



:go_check

pushd "server\work\dfo-lan"

"..\..\..\tools\go\bin\go.exe" run ./cmd/dfolauncher check --root "%~dp0.." %*

rem Read outside any parenthesised block so this is the command's own exit code.

set "GO_CHECK_RC=%ERRORLEVEL%"

popd

exit /b 0


@echo off
chcp 65001 >nul
setlocal
cd /d "%~dp0..\.."
if not defined VCVARS64 set "VCVARS64=C:\Program Files (x86)\Microsoft Visual Studio\2019\Community\VC\Auxiliary\Build\vcvars64.bat"
if not exist "%VCVARS64%" (
  echo Set VCVARS64 to the x64 MSVC environment script.
  exit /b 1
)
call "%VCVARS64%"
if not "%ERRORLEVEL%"=="0" exit /b %ERRORLEVEL%
if defined US115_SDK_ROOT (
  if not defined US115_SDK_VERSION set "US115_SDK_VERSION=10.0.26100.0"
)
if defined US115_SDK_ROOT (
  set "INCLUDE=%US115_SDK_ROOT%\microsoft.windows.sdk.cpp\c\Include\%US115_SDK_VERSION%\ucrt;%US115_SDK_ROOT%\microsoft.windows.sdk.cpp\c\Include\%US115_SDK_VERSION%\shared;%US115_SDK_ROOT%\microsoft.windows.sdk.cpp\c\Include\%US115_SDK_VERSION%\um;%INCLUDE%"
  set "LIB=%US115_SDK_ROOT%\microsoft.windows.sdk.cpp.x64\c\ucrt\x64;%US115_SDK_ROOT%\microsoft.windows.sdk.cpp.x64\c\um\x64;%LIB%"
)
if not exist "%~dp0dist" mkdir "%~dp0dist"
cd /d "%~dp0dist"
cl /nologo /std:c++17 /O2 /MT /EHsc /utf-8 /LD /I . /I ..\src ..\src\pilot.cpp /link /OUT:EliteCompanionPilot.dll psapi.lib
if not "%ERRORLEVEL%"=="0" exit /b %ERRORLEVEL%
cl /nologo /std:c++17 /O2 /MT /EHsc /utf-8 /I . /I ..\src ..\src\test_inline.cpp /Fe:test_inline.exe
if not "%ERRORLEVEL%"=="0" exit /b %ERRORLEVEL%
test_inline.exe
if not "%ERRORLEVEL%"=="0" exit /b %ERRORLEVEL%
cl /nologo /std:c++17 /O2 /MT /EHsc /utf-8 /I . /I ..\src ..\src\test_policy.cpp /Fe:test_policy.exe
if not "%ERRORLEVEL%"=="0" exit /b %ERRORLEVEL%
test_policy.exe
if not "%ERRORLEVEL%"=="0" exit /b %ERRORLEVEL%
cl /nologo /std:c++17 /O2 /MT /EHsc /utf-8 /I . /I ..\src ..\src\test_crash_trace.cpp /Fe:test_crash_trace.exe psapi.lib
if not "%ERRORLEVEL%"=="0" exit /b %ERRORLEVEL%
test_crash_trace.exe
if not "%ERRORLEVEL%"=="0" exit /b %ERRORLEVEL%
cl /nologo /std:c++17 /O2 /MT /EHsc /utf-8 /I . /I ..\src ..\src\test_companion_resource_guard.cpp /Fe:test_companion_resource_guard.exe
if not "%ERRORLEVEL%"=="0" exit /b %ERRORLEVEL%
test_companion_resource_guard.exe
if not "%ERRORLEVEL%"=="0" exit /b %ERRORLEVEL%
cl /nologo /std:c++17 /O2 /MT /EHsc /utf-8 /I . /I ..\src ..\src\test_native_room_scope.cpp /Fe:test_native_room_scope.exe
if not "%ERRORLEVEL%"=="0" exit /b %ERRORLEVEL%
test_native_room_scope.exe
if not "%ERRORLEVEL%"=="0" exit /b %ERRORLEVEL%
cl /nologo /std:c++17 /O2 /MT /EHsc /utf-8 /I . /I ..\src ..\src\test_native_panel_binding.cpp /Fe:test_native_panel_binding.exe
if not "%ERRORLEVEL%"=="0" exit /b %ERRORLEVEL%
test_native_panel_binding.exe
if not "%ERRORLEVEL%"=="0" exit /b %ERRORLEVEL%
cl /nologo /std:c++17 /O2 /MT /EHsc /utf-8 /I . /I ..\src ..\src\test_native_registration_scope.cpp /Fe:test_native_registration_scope.exe
if not "%ERRORLEVEL%"=="0" exit /b %ERRORLEVEL%
test_native_registration_scope.exe
if not "%ERRORLEVEL%"=="0" exit /b %ERRORLEVEL%
cl /nologo /std:c++17 /O2 /MT /EHsc /utf-8 /I . /I ..\src ..\src\test_all_policy.cpp /Fe:test_all_policy.exe
if not "%ERRORLEVEL%"=="0" exit /b %ERRORLEVEL%
test_all_policy.exe
if not "%ERRORLEVEL%"=="0" exit /b %ERRORLEVEL%
cl /nologo /std:c++17 /O2 /MT /EHsc /utf-8 /I . /I ..\src ..\src\test_roster_key.cpp /Fe:test_roster_key.exe
if not "%ERRORLEVEL%"=="0" exit /b %ERRORLEVEL%
test_roster_key.exe
if not "%ERRORLEVEL%"=="0" exit /b %ERRORLEVEL%
cl /nologo /std:c++17 /O2 /MT /EHsc /utf-8 /I . /I ..\src ..\src\test_exit_trace.cpp /Fe:test_exit_trace.exe
if not "%ERRORLEVEL%"=="0" exit /b %ERRORLEVEL%
test_exit_trace.exe
if not "%ERRORLEVEL%"=="0" exit /b %ERRORLEVEL%
cl /nologo /std:c++17 /O2 /MT /EHsc /utf-8 /I . /I ..\src ..\src\test_graphics_failure_trace.cpp /Fe:test_graphics_failure_trace.exe psapi.lib
if not "%ERRORLEVEL%"=="0" exit /b %ERRORLEVEL%
test_graphics_failure_trace.exe
if not "%ERRORLEVEL%"=="0" exit /b %ERRORLEVEL%
exit /b 0

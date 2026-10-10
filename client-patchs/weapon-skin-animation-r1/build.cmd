@echo off
setlocal
chcp 65001 >nul
cd /d "%~dp0..\.."
if not defined VCVARS64 set "VCVARS64=C:\Program Files (x86)\Microsoft Visual Studio\2019\Community\VC\Auxiliary\Build\vcvars64.bat"
call "%VCVARS64%"
if not "%ERRORLEVEL%"=="0" exit /b %ERRORLEVEL%
if not defined US115_SDK_VERSION set "US115_SDK_VERSION=10.0.26100.0"
if defined US115_SDK_ROOT set "INCLUDE=%US115_SDK_ROOT%\microsoft.windows.sdk.cpp\c\Include\%US115_SDK_VERSION%\ucrt;%US115_SDK_ROOT%\microsoft.windows.sdk.cpp\c\Include\%US115_SDK_VERSION%\shared;%US115_SDK_ROOT%\microsoft.windows.sdk.cpp\c\Include\%US115_SDK_VERSION%\um;%INCLUDE%"
if defined US115_SDK_ROOT set "LIB=%US115_SDK_ROOT%\microsoft.windows.sdk.cpp.x64\c\ucrt\x64;%US115_SDK_ROOT%\microsoft.windows.sdk.cpp.x64\c\um\x64;%LIB%"
if not exist "%~dp0dist" mkdir "%~dp0dist"
cd /d "%~dp0dist"
cl /nologo /std:c++17 /O2 /MT /EHsc /utf-8 /LD /I ..\src ..\src\plugin.cpp /link /OUT:WeaponSkinAnimation.dll
if not "%ERRORLEVEL%"=="0" exit /b %ERRORLEVEL%
cl /nologo /std:c++17 /O2 /MT /EHsc /utf-8 /I ..\src ..\src\test_policy.cpp /Fe:test_policy.exe
if not "%ERRORLEVEL%"=="0" exit /b %ERRORLEVEL%
test_policy.exe
if not "%ERRORLEVEL%"=="0" exit /b %ERRORLEVEL%
cl /nologo /std:c++17 /O2 /MT /EHsc /utf-8 /I ..\src ..\src\test_native.cpp /Fe:test_native.exe
if not "%ERRORLEVEL%"=="0" exit /b %ERRORLEVEL%
test_native.exe
if not "%ERRORLEVEL%"=="0" exit /b %ERRORLEVEL%
cl /nologo /std:c++17 /O2 /MT /EHsc /utf-8 /I ..\src ..\src\test_hooks.cpp /Fe:test_hooks.exe
if not "%ERRORLEVEL%"=="0" exit /b %ERRORLEVEL%
test_hooks.exe
if not "%ERRORLEVEL%"=="0" exit /b %ERRORLEVEL%
cl /nologo /std:c++17 /O2 /MT /EHsc /utf-8 /I ..\src ..\src\test_host.cpp /Fe:test_host.exe
if not "%ERRORLEVEL%"=="0" exit /b %ERRORLEVEL%
test_host.exe WeaponSkinAnimation.dll inactive
if not "%ERRORLEVEL%"=="0" exit /b %ERRORLEVEL%
test_host.exe WeaponSkinAnimation.dll enabled
if not "%ERRORLEVEL%"=="0" exit /b %ERRORLEVEL%
exit /b 0

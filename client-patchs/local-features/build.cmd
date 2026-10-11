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
cl /nologo /std:c++17 /O2 /MT /EHsc /utf-8 /DUNICODE /D_UNICODE /LD ..\src\local_features.cpp /link user32.lib gdi32.lib comctl32.lib iphlpapi.lib ws2_32.lib bcrypt.lib /OUT:LocalFeatures.dll
if not "%ERRORLEVEL%"=="0" exit /b %ERRORLEVEL%
cl /nologo /std:c++17 /O2 /MT /EHsc /utf-8 ..\src\test_protocol.cpp /Fe:test_protocol.exe
if not "%ERRORLEVEL%"=="0" exit /b %ERRORLEVEL%
if "%~1"=="" (test_protocol.exe "%~dp0src\testdata\feature-controller-view.bin") else (test_protocol.exe "%~1")
if not "%ERRORLEVEL%"=="0" exit /b %ERRORLEVEL%
cl /nologo /std:c++17 /O2 /MT /EHsc /utf-8 ..\src\test_symbol_page.cpp /Fe:test_symbol_page.exe /link user32.lib
if not "%ERRORLEVEL%"=="0" exit /b %ERRORLEVEL%
test_symbol_page.exe
exit /b %ERRORLEVEL%

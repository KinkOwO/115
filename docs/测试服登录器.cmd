@echo off
setlocal
title DFO 账号工具

rem 改这三行即可：账号服务地址、游戏网关地址与端口
set "SERVER=220.166.147.216:7301"
set "GAME_IP=220.166.147.216"
set "GAME_PORT=7001"

set "CLIENT_DIR=%~2"
if "%CLIENT_DIR%"=="" set "CLIENT_DIR=%~dp0"
set "ACCT_TMP=%TEMP%\dfo-acct-%RANDOM%.json"
set "ACCT_CODE=%TEMP%\dfo-acct-%RANDOM%.code"
set "USER=%~3"
set "PASS=%~4"
set "TICKET="
set "CODE="
set "BODY="

where curl >nul 2>nul
if errorlevel 1 (
  echo   [错误] 需要 Windows 10 自带的 curl.exe
  goto :done
)

if /i "%~1"=="1" goto :register
if /i "%~1"=="register" goto :register
if /i "%~1"=="2" goto :login
if /i "%~1"=="login" goto :login
if /i "%~1"=="3" goto :check
if /i "%~1"=="check" goto :check
if /i "%~1"=="help" goto :help
if /i "%~1"=="-h" goto :help
if /i "%~1"=="0" exit /b 0

set "MENU=1"

:menu
cls
echo.
echo    DFO 账号工具
echo    --------------------------------------------
echo      账号服务   %SERVER%
echo      游戏网关   %GAME_IP%:%GAME_PORT%
echo    --------------------------------------------
echo.
echo      [1]   注册新账号
echo      [2]   登录并启动游戏
echo      [3]   只登录校验
echo      [0]   退出
echo.
set "CHOICE="
set /p "CHOICE=   请选择："
echo.
if "%CHOICE%"=="1" set "USER=" & set "PASS=" & set "TICKET=" & goto :register
if "%CHOICE%"=="2" set "USER=" & set "PASS=" & set "TICKET=" & goto :login
if "%CHOICE%"=="3" set "USER=" & set "PASS=" & set "TICKET=" & goto :check
if "%CHOICE%"=="0" exit /b 0
goto :menu

:register
call :credentials
if not defined USER goto :end
echo   [信息] 正在注册 %USER% ...
call :post "/api/account/register"
if "%CODE%"=="201" goto :registered
if "%CODE%"=="409" (
  echo   [失败] 这个账号名已经被占用了
  goto :end
)
if "%CODE%"=="400" (
  echo   [失败] 账号名或密码不符合要求（账号 3-32 位，密码 8-64 位不含空格）
  goto :end
)
if "%CODE%"=="" goto :offline
if "%CODE%"=="000" goto :offline
echo   [失败] 注册失败（HTTP %CODE%）
goto :end

:registered
echo   [成功] 账号 %USER% 注册完成
echo.
set "ANSWER="
set /p "ANSWER=   现在登录并启动游戏吗？(Y/n) "
if /i "%ANSWER%"=="n" goto :end
if /i "%ANSWER%"=="no" goto :end
goto :login

:login
call :credentials
if not defined USER goto :end
call :signin
if not defined TICKET goto :end
goto :launch

:check
call :credentials
if not defined USER goto :end
call :signin
if not defined TICKET goto :end
echo   [提示] 只做了登录校验，没有启动游戏
goto :end

:launch
if not exist "%CLIENT_DIR%DFO.exe" (
  echo   [失败] 找不到 %CLIENT_DIR%DFO.exe，请把本脚本放到客户端目录
  goto :end
)
echo   [信息] 正在启动客户端 ...
cd /d "%CLIENT_DIR%"
start "" "DFO.exe" "3?%GAME_IP%?%GAME_PORT%?%USER%?%TICKET%?0?0?30?0?0?0"
echo   [成功] 客户端已启动（同账号在别处登录会顶掉本机）
goto :end

:credentials
if not defined USER set /p "USER=   账号名："
if not defined USER (
  echo   [失败] 账号名不能为空
  goto :eof
)
if not defined PASS for /f "usebackq delims=" %%p in (`powershell -NoProfile -Command "$s=Read-Host '   密码（输入时不显示）' -AsSecureString; [Runtime.InteropServices.Marshal]::PtrToStringAuto([Runtime.InteropServices.Marshal]::SecureStringToBSTR($s))"`) do set "PASS=%%p"
if not defined PASS (
  echo   [失败] 密码不能为空
  set "USER="
  goto :eof
)
goto :eof

:signin
set "TICKET="
echo   [信息] 正在登录 %USER% ...
call :post "/api/account/login"
if "%CODE%"=="200" goto :signed
if "%CODE%"=="401" (
  echo   [失败] 账号或密码不正确
  goto :eof
)
if "%CODE%"=="400" (
  echo   [失败] 账号名或密码不符合要求
  goto :eof
)
if "%CODE%"=="503" (
  echo   [失败] 账号服务暂不可用（PostgreSQL / Redis 没启动？）
  goto :eof
)
if "%CODE%"=="" goto :offline
if "%CODE%"=="000" goto :offline
echo   [失败] 登录失败（HTTP %CODE%）
goto :eof

:signed
for /f "tokens=2 delims=:," %%t in ('findstr /i "ticket" "%ACCT_TMP%"') do set "TICKET=%%~t"
if not defined TICKET (
  echo   [失败] 账号服务没有返回票据
  goto :eof
)
echo   [成功] 已登录：%USER%
goto :eof

:post
set "CODE="
set "BODY="
del "%ACCT_TMP%" >nul 2>nul
del "%ACCT_CODE%" >nul 2>nul
curl -s -S -m 15 -H "Content-Type: application/json" -d "{\"username\":\"%USER%\",\"password\":\"%PASS%\"}" -o "%ACCT_TMP%" -w "%%{http_code}" "http://%SERVER%%~1" > "%ACCT_CODE%"
if exist "%ACCT_CODE%" set /p CODE=<"%ACCT_CODE%"
if exist "%ACCT_TMP%" set /p BODY=<"%ACCT_TMP%"
goto :eof

:offline
echo   [失败] 连不上账号服务 %SERVER%（先运行 服务端）
goto :eof

:help
echo.
echo   用法：登录启动游戏.cmd [1^|2^|3^|help] [客户端目录] [账号] [密码]
echo.
echo     1 / register   注册新账号
echo     2 / login      登录并启动游戏
echo     3 / check      只登录校验
echo     help           显示本帮助
echo.
echo   不带参数运行会显示菜单。
goto :done

:end
echo.
if defined MENU pause
if defined MENU goto :menu
goto :done

:done
del "%ACCT_TMP%" >nul 2>nul
del "%ACCT_CODE%" >nul 2>nul
echo.
pause
exit /b 0

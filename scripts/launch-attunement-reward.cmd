@echo off
chcp 65001 >nul
cd /d "%~dp0.."
title DFO 115us - launcher (optional NOTI 2859 diagnostic)
rem ===========================================================================
rem  Two modes:
rem    no arguments  -> PLAIN launch, no injection. The server's own wiring
rem                     sends NOTI 2859 with the run's rolled grade
rem                     (see cmd/wireprobe/attunement_reward.go).
rem    with a value  -> diagnostic override for this run:
rem                     launch-attunement-reward.cmd 42,42,42
rem                     launch-attunement-reward.cmd "42,42,42"
rem                     launch-attunement-reward.cmd 2c0000002b00000000000000
rem                     launch-attunement-reward.cmd @runtime\attunement-reward.txt
rem                     launch-attunement-reward.cmd 42,42,42 --server-only
rem
rem  Payload must be exactly 12 bytes = 3 x u32 (24 hex digits also accepted).
rem  @file is re-read on EVERY dungeon entry, so a probe series needs no restart.
rem  The payload is sent on EVERY dungeon entry while armed - test in the abyss.
rem
rem  Why an environment variable and not a flag: the Go launcher's `launch`
rem  subcommand has a fixed flag set and does NOT forward unknown flags to the
rem  gateway, but the gateway child inherits this process environment
rem  (internal/launcher/serverrun.go -> BuildServerEnv(CurrentEnv(), ...),
rem  and CurrentEnv() is os.Environ()).
rem
rem  NOTE (2026-10-08): cmd treats BOTH comma and space as argument separators,
rem  so an unquoted 42,42,42 arrives as three tokens and the server would see
rem  only "42". The rejoin block below puts them back together.
rem
rem  It lives in scripts\ (the repo forbids .cmd in the root, AGENTS.md SS0.4.1)
rem  and walks back one level to the repo root, so every path it touches stays
rem  ASCII and it never depends on how cmd.exe decodes non-ASCII bytes.
rem ===========================================================================

set "SPEC="
set "FWD="

if "%~1"=="" (
    echo [launch] mode   = PLAIN ^(no diagnostic override^)
    echo [launch] note   = NOTI 2859 is sent by the server with the run grade
    echo.
) else (
    set "SPEC=%~1"
    set "FWD=%2 %3"
    rem  rejoin the comma-split form: %1..%3 all numeric-ish, launcher args start at %4
    if not "%~2"=="" if not "%~3"=="" (
        echo %~2| findstr /r "^[0-9-][0-9]*$" >nul && (
            echo %~3| findstr /r "^[0-9-][0-9]*$" >nul && (
                set "SPEC=%~1,%~2,%~3"
                set "FWD=%4"
            )
        )
    )
    set "DFO_ATTUNEMENT_REWARD=%SPEC%"
    echo [inject] payload= %SPEC%
    echo [inject] expect : attunement reward ^(noti 2859^): ... in the startup log
    echo.
)

echo [1/2] restore local build snapshot...
for %%F in (wireprobe-pvf.exe dfolauncher.exe) do (
    copy /y "runtime\dev-local\%%F" "server\work\dfo-lan\bin\%%F" >nul || goto :fail
)
copy /y "runtime\dev-local\pvf-default.json"  "server\work\dfo-lan\configs\pvf-default.json" >nul || goto :fail
copy /y "runtime\dev-local\config_help.json"  "server\work\dfo-lan\cmd\wireprobe\testdata\config_help.json" >nul || goto :fail
copy /y "runtime\dev-local\config_legacy.json" "server\work\dfo-lan\cmd\wireprobe\testdata\config_legacy.json" >nul || goto :fail
echo     done.

echo [2/2] launch (CLI launcher)...
server\work\dfo-lan\bin\dfolauncher.exe launch --root . %FWD%
set RC=%errorlevel%
echo.
echo exit=%RC%
pause
exit /b %RC%

:fail
echo [error] copy failed - close the game / server first (files are locked).
pause
exit /b 1

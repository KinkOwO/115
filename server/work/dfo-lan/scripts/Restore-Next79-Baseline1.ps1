$ErrorActionPreference = 'Stop'
$next79Root = Split-Path -Parent $PSScriptRoot
$next79Backup = Join-Path $next79Root 'runtime/baselines/next79-baseline1-unlimited/wireprobe-pvf.exe'
$next79Expected = '854cbe1d8c1738f6ff46f6ad1a356d614fa44900e77f5d284589a5a23c8720c9'
if (Get-Process -Name 'wireprobe-pvf','wireprobe-handoff-source' -ErrorAction SilentlyContinue) { throw 'Stop the game server before restoring baseline1.' }
if ((Get-FileHash -LiteralPath $next79Backup -Algorithm SHA256).Hash.ToLowerInvariant() -ne $next79Expected) { throw 'Baseline1 checksum mismatch; refusing restore.' }
Copy-Item -LiteralPath $next79Backup -Destination (Join-Path $next79Root 'bin/wireprobe-pvf.exe') -Force
Copy-Item -LiteralPath $next79Backup -Destination (Join-Path $next79Root 'bin/wireprobe-handoff-source.exe') -Force
& (Join-Path $PSScriptRoot 'Set-Ispins-Mode.ps1') -Mode unlimited
Write-Output 'Baseline1 restored. Player saves and client resources were preserved. Start the server manually.'

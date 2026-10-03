param([Parameter(Mandatory=$true)][ValidateSet('unlimited','weekly')][string]$Mode)
$ErrorActionPreference = 'Stop'
$next79ModuleRoot = Split-Path -Parent $PSScriptRoot
$next79Profile = Join-Path $next79ModuleRoot 'configs/pvf-default.json'
$next79Config = Get-Content -LiteralPath $next79Profile -Raw -Encoding UTF8 | ConvertFrom-Json
if ($null -eq $next79Config.environment) { throw 'Missing profile environment' }
$next79Config.environment | Add-Member -NotePropertyName 'DFO_ISPINS_MODE' -NotePropertyValue $Mode -Force
$next79Json = $next79Config | ConvertTo-Json -Depth 40
[System.IO.File]::WriteAllText($next79Profile, $next79Json, [System.Text.UTF8Encoding]::new($false))
Write-Output "Ispins mode saved: $Mode. Restart the server to apply. Player saves are unchanged."

# Local native-PVF test runner.
#
# One command for every case — no environment variables to remember. It points
# DFO_PVF_CORE_TEST_ARCHIVE at the current inner PVF, enables the disposable
# metadata cache, and runs the Go tests. A single checksum-verified archive is
# opened per package and reused via runtime/pvf-cache instead of re-parsing the
# whole inner PVF for every test.
#
# Usage:
#   pwsh -File scripts/test_local.ps1                 # fast default: go test ./...
#   pwsh -File scripts/test_local.ps1 -Full           # run the gated heavy/exhaustive tests too
#   pwsh -File scripts/test_local.ps1 ./internal/dungeon
#   pwsh -File scripts/test_local.ps1 -Run TestFoo ./internal/catalog
#
# Default keeps every test case under ~30s: sampled parity tests run on a
# deterministic cross-range sample and the heaviest archive tests skip. -Full
# restores exhaustive sweeps (sets DFO_PVF_ARCHIVE_FULL_SWEEP=1).
[CmdletBinding()]
param(
    [string]$Run = "",
    [switch]$Full,
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$GoTestArgs
)

$ErrorActionPreference = 'Stop'

$moduleRoot = Split-Path -Parent $PSScriptRoot

if (-not $env:DFO_PVF_CORE_TEST_ARCHIVE) {
    $archive = Join-Path $moduleRoot '..' 'client-build' 'Script.inner.pvf'
    if (-not (Test-Path -LiteralPath $archive)) {
        throw "inner PVF not found at '$archive'; set DFO_PVF_CORE_TEST_ARCHIVE explicitly"
    }
    $env:DFO_PVF_CORE_TEST_ARCHIVE = (Resolve-Path -LiteralPath $archive).Path
}
if (-not $env:DFO_PVF_CACHE_DIR) {
    $env:DFO_PVF_CACHE_DIR = 'runtime/pvf-cache'
}
if ($Full) {
    $env:DFO_PVF_ARCHIVE_FULL_SWEEP = '1'
}

if (-not $GoTestArgs -or $GoTestArgs.Count -eq 0) {
    $GoTestArgs = @('./...')
}

$goArgs = @('test')
if ($Run) {
    $goArgs += @('-run', $Run)
}
$goArgs += $GoTestArgs

Write-Host "DFO_PVF_CORE_TEST_ARCHIVE=$env:DFO_PVF_CORE_TEST_ARCHIVE"
Write-Host "DFO_PVF_CACHE_DIR=$env:DFO_PVF_CACHE_DIR"
Write-Host "DFO_PVF_ARCHIVE_FULL_SWEEP=$env:DFO_PVF_ARCHIVE_FULL_SWEEP"
Write-Host "go $($goArgs -join ' ')"

Push-Location $moduleRoot
try {
    & go @goArgs
    exit $LASTEXITCODE
}
finally {
    Pop-Location
}

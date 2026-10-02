# Local native-PVF test runner.
#
# Points DFO_PVF_CORE_TEST_ARCHIVE at the current inner PVF and enables the
# disposable metadata cache, then runs the Go tests. This keeps a single
# checksum-verified archive open per package and reuses runtime/pvf-cache
# between runs instead of re-parsing the whole inner PVF for every test.
#
# Usage:
#   pwsh -File scripts/test_local.ps1                 # go test ./...
#   pwsh -File scripts/test_local.ps1 ./internal/dungeon
#   pwsh -File scripts/test_local.ps1 -Run TestFoo ./internal/catalog
#
# Release verification still uses the serial, cache-disabled form:
#   go test -p 1 -count=1 ./...
[CmdletBinding()]
param(
    [string]$Run = "",
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
Write-Host "go $($goArgs -join ' ')"

Push-Location $moduleRoot
try {
    & go @goArgs
    exit $LASTEXITCODE
}
finally {
    Pop-Location
}

param([string]$Go = 'go')
$ErrorActionPreference = 'Stop'
Push-Location (Join-Path $PSScriptRoot 'work/dfo-lan')
try {
    & $Go test ./...
    if ($LASTEXITCODE -ne 0) { throw 'Tests failed' }
    & $Go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'Vet failed' }
    & $Go build -trimpath -o bin/wireprobe-handoff-source.exe ./cmd/wireprobe
    if ($LASTEXITCODE -ne 0) { throw 'Build failed' }
    Write-Output 'Built source candidate. Use Start-DFO.cmd --source-build to test it; archived39 was preserved.'
} finally { Pop-Location }

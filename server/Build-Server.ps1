param([string]$Go = 'go', [switch]$UpdatePVFDefault, [switch]$CheckSQL)
$ErrorActionPreference = 'Stop'
Push-Location (Join-Path $PSScriptRoot 'work/dfo-lan')
try {
    # Normal builds use checked-in generated Go. SQL authors opt into the
    # pinned generator check; this does not require a database connection.
    if ($CheckSQL) {
        & ./scripts/Generate-SQL.ps1 -Check
    }
    & $Go test ./...
    if ($LASTEXITCODE -ne 0) { throw 'Tests failed' }
    & $Go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'Vet failed' }
    & $Go build -trimpath -o bin/wireprobe-handoff-source.exe ./cmd/wireprobe
    if ($LASTEXITCODE -ne 0) { throw 'Build failed' }
    if ($UpdatePVFDefault -or -not (Test-Path -LiteralPath bin/wireprobe-pvf.exe)) {
        Copy-Item -LiteralPath bin/wireprobe-handoff-source.exe -Destination bin/wireprobe-pvf.exe -Force
        Write-Output 'Built source and default PVF gateway; archived39 was preserved.'
    } else {
        Write-Output 'Built source candidate; confirmed PVF gateway was preserved. Use -UpdatePVFDefault to publish an accepted build.'
    }
} finally { Pop-Location }

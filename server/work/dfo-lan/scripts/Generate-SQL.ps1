param([switch]$Check)
$ErrorActionPreference = 'Stop'
$moduleRoot = Split-Path $PSScriptRoot -Parent
Push-Location $moduleRoot
try {
    $version = (& sqlc version).Trim()
    if ($LASTEXITCODE -ne 0 -or $version -ne 'v1.31.1') {
        throw 'Use sqlc v1.31.1 to generate database access code.'
    }
    if (-not $Check) {
        & sqlc generate
        if ($LASTEXITCODE -ne 0) { throw 'sqlc generation failed' }
        return
    }
    # Generate into ignored scratch space. Checking never rewrites checked-in
    # generated code, and requires no connection to any database.
    $scratch = Join-Path $moduleRoot ('.tmp/sqlc-check/' + [guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $scratch -Force | Out-Null
    $config = Get-Content -LiteralPath (Join-Path $moduleRoot 'sqlc.yaml') -Raw -Encoding utf8
    $config = $config.Replace('internal/database/sql/postgres/migrations', '../../../internal/database/sql/postgres/migrations')
    $config = $config.Replace('internal/database/sql/postgres/queries', '../../../internal/database/sql/postgres/queries')
    $config = $config.Replace('internal/database/sqlcgen', 'generated')
    $configPath = Join-Path $scratch 'sqlc.yaml'
    [System.IO.File]::WriteAllText($configPath, $config, [System.Text.UTF8Encoding]::new($false))
    & sqlc generate -f $configPath
    if ($LASTEXITCODE -ne 0) { throw 'sqlc generation check failed' }
    $actual = Join-Path $moduleRoot 'internal/database/sqlcgen'
    $expected = Join-Path $scratch 'generated'
    $names = @(Get-ChildItem -LiteralPath $actual -Filter '*.go' | ForEach-Object Name)
    $expectedNames = @(Get-ChildItem -LiteralPath $expected -Filter '*.go' | ForEach-Object Name)
    if (Compare-Object $names $expectedNames) { throw 'Generated file list differs; run scripts/Generate-SQL.ps1.' }
    foreach ($name in $names) {
        $a = (Get-FileHash -LiteralPath (Join-Path $actual $name)).Hash
        $b = (Get-FileHash -LiteralPath (Join-Path $expected $name)).Hash
        if ($a -ne $b) { throw "Generated code differs: $name; run scripts/Generate-SQL.ps1." }
    }
    Write-Output 'sqlc v1.31.1 generated code is up to date.'
} finally {
    Pop-Location
}

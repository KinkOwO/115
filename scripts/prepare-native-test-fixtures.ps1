# Export parser regression fixtures only; runtime gameplay continues to read PVF.
[CmdletBinding()]
param([string]$Archive, [string]$GoExecutable = 'go')
$ErrorActionPreference = 'Stop'
$moduleRoot = Join-Path (Split-Path -Parent $PSScriptRoot) 'server/work/dfo-lan'
$archivePath = (Resolve-Path -LiteralPath $Archive).Path
$checksum = (Get-FileHash -LiteralPath $archivePath -Algorithm SHA256).Hash.ToLowerInvariant()
$fixtureRoot = Join-Path $moduleRoot 'runtime/test/source-exports'
[IO.Directory]::CreateDirectory($fixtureRoot) | Out-Null
$tool = Join-Path $fixtureRoot 'dfo-tool.exe'
Push-Location $moduleRoot
try {
    & $GoExecutable build -o $tool ./cmd/dfo-tool
    if ($LASTEXITCODE -ne 0) { throw 'dfo-tool build failed' }
    $entries = @(
        'event/eventgift.evt',
        'live/event/kor/2026/0326_boostup/boostup.evt',
        'live/event/kor/2026/0326_boostup/boostupspecupchallenge.evt',
        'etc/variousbufflist.etc',
        'character/swordman/dsswordman.chr',
        'character/mage/creatormage.chr',
        'contents/2025/equipmentsetjournal/etc/equipmentsetjournal.cos',
        'etc/115lvability/equipmenttransformsystem.cos',
        'etc/115lvability/setpointinfo.cos',
        'etc/115lvability2/oathpointinfo.cos'
    )
    & $tool pvfinspect -source $archivePath -output $fixtureRoot -files ($entries -join ',') -tokens
    if ($LASTEXITCODE -ne 0) { throw 'Native source export failed' }
    $copies = @{
        '02-boostupspecupchallenge.evt.tokens.json' = 'challenge/00-boostupspecupchallenge.evt.tokens.json'
        '03-variousbufflist.etc.tokens.json' = 'buff/00-variousbufflist.etc.tokens.json'
        '04-dsswordman.chr.tokens.json' = 'special/00-dsswordman.chr.tokens.json'
        '05-creatormage.chr.tokens.json' = 'special/01-creatormage.chr.tokens.json'
        '06-equipmentsetjournal.cos.txt' = '../../pvf-journal/00-equipmentsetjournal.cos.txt'
        '07-equipmenttransformsystem.cos.txt' = '../../pvfinspect-primer/lv1/00-equipmenttransformsystem.cos.txt'
        '08-setpointinfo.cos.txt' = '../../pvfinspect-primer/lv1/10-setpointinfo.cos.txt'
        '09-oathpointinfo.cos.txt' = '../../pvfinspect-primer/lv1/11-oathpointinfo.cos.txt'
    }
    foreach ($file in $copies.Keys) {
        $destination = [IO.Path]::GetFullPath((Join-Path $fixtureRoot $copies[$file]))
        $runtimeRoot = [IO.Path]::GetFullPath((Join-Path $moduleRoot 'runtime')) + [IO.Path]::DirectorySeparatorChar
        if (-not $destination.StartsWith($runtimeRoot, [StringComparison]::OrdinalIgnoreCase)) { throw "Fixture output escaped runtime: $destination" }
        [IO.Directory]::CreateDirectory([IO.Path]::GetDirectoryName($destination)) | Out-Null
        Copy-Item -LiteralPath (Join-Path $fixtureRoot $file) -Destination $destination
    }
    # Publish readiness only after every required source entry was exported.
    @{Archive = $archivePath; ArchiveSHA256 = $checksum} | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $fixtureRoot 'ready.json') -Encoding utf8
}
finally { Pop-Location }

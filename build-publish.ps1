# build-publish.ps1 - DFO 115us ?????????(????,???????)
# ??: ????.cmd                       ???? git ????
#       ????.cmd -Branch release/minimal-publish   ????
#       ????.cmd -OutDir D:\out        ??????(???????)
# ??: git ???? -> ?? tools/{python,pg}(?? pgAdmin 4) ->
#       launcher.local.json ??(????) -> Python ?? zip(???/UTF-8) -> ??????
[CmdletBinding()]
param(
    [string]$Branch = '',
    [string]$OutDir = ''
)
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.IO.Compression.FileSystem

$ROOT = Split-Path -Parent $MyInvocation.MyCommand.Path
if (-not $Branch) { $Branch = 'release/minimal-publish' }
if (-not $OutDir) { $OutDir = Split-Path -Parent $ROOT }

git -C $ROOT rev-parse --verify --quiet $Branch 2>$null | Out-Null
if ($LASTEXITCODE -ne 0) { throw "?????: $Branch" }

$stamp = Get-Date -Format 'yyyyMMdd'
$zipOut = Join-Path $OutDir "DFO-115US-?????-$stamp.zip"
$work = Join-Path $env:TEMP "df-publish-$stamp"
$tar  = Join-Path $env:TEMP "df-publish-$stamp.tar.zip"

Write-Host "== ?????: ?? [$Branch] =="
Write-Host ("   ??: {0}" -f $zipOut)

if (Test-Path $work) { Remove-Item $work -Recurse -Force }
New-Item -ItemType Directory -Path $work | Out-Null
try {
    # 1. ? git ????????(????????)
    Write-Host '[1/5] git ????...'
    git -C $ROOT archive --format=zip -o $tar $Branch
    if ($LASTEXITCODE -ne 0) { throw 'git archive ??' }
    [IO.Compression.ZipFile]::ExtractToDirectory($tar, $work)

    # 2. ????????(python/pg),?? pgAdmin 4
    Write-Host '[2/5] ????????...'
    $tools = Join-Path $ROOT 'tools'
    New-Item -ItemType Directory -Path (Join-Path $work 'tools') | Out-Null
    foreach ($d in @('python','pg')) {
        $src = Join-Path $tools $d
        if (-not (Test-Path $src)) { Write-Warning "???????? tools\$d (??)"; continue }
        Copy-Item $src (Join-Path $work "tools\$d") -Recurse -Force
    }
    $pga = Join-Path $work 'tools\pg\pgsql\pgAdmin 4'
    if (Test-Path $pga) { Remove-Item $pga -Recurse -Force; Write-Host '   ??? pgAdmin 4' }

    # 3. launcher.local.json ??(????)
    Write-Host '[3/5] ?? launcher.local.json ??...'
    Copy-Item (Join-Path $ROOT 'server\launcher.example.json') (Join-Path $work 'server\launcher.local.json') -Force

    # 4. Python ?? zip ??(???????UTF-8 ????? ./ ??)
    Write-Host '[4/5] ??? zip...'
    $py = Join-Path $ROOT 'tools\python\python.exe'
    if (-not (Test-Path $py)) { throw '?? tools\python\python.exe' }
    $pyScript = Join-Path $ROOT 'scripts\build_publish_zip.py'
    & $py $pyScript $work $zipOut
    if ($LASTEXITCODE -ne 0) { throw 'zip ????' }

    # 5. ??????
    Write-Host '[5/5] ??????...'
    $z = [IO.Compression.ZipFile]::OpenRead($zipOut)
    try {
        $need = @('????.cmd','server/launcher.local.json','server/work/dfo-lan/bin/wireprobe-dungeon39.exe','server/work/dfo-lan/bin/wireprobe-handoff-source.exe','server/work/dfo-lan/configs/items.index.json')
        # Ship what is bundled: a sqlite-only package carries neither the PostgreSQL
        # portable tree nor (once the launcher is fully Go) the Python runtime, and the
        # check must not demand files the package deliberately omits.
        if (Test-Path (Join-Path $ROOT 'tools\pg')) { $need += 'tools/pg/pgsql/bin/initdb.exe' }
        if (Test-Path (Join-Path $ROOT 'tools\python')) { $need += 'tools/python/python.exe' }
        $miss = @()
        foreach ($n in $need) {
            $hit = $z.Entries | Where-Object { $_.FullName -eq $n }
            if (-not $hit) { $miss += $n }
        }
        if ($miss.Count -gt 0) { Remove-Item $zipOut -Force -ErrorAction SilentlyContinue; throw "?????????: $($miss -join ', ')" }
        Write-Host ("   ????: {0} ??" -f $z.Entries.Count)
    } finally { $z.Dispose() }

    $mb = [math]::Round((Get-Item $zipOut).Length / 1MB, 1)
    Write-Host ''
    Write-Host ("[??] ??????: {0}  ({1} MB)" -f $zipOut, $mb)
    Write-Host '???: ?????? ????.cmd ???, ? ?????.cmd / ????.cmd?'
} finally {
    Remove-Item $work -Recurse -Force -ErrorAction SilentlyContinue
    Remove-Item $tar -Force -ErrorAction SilentlyContinue
}

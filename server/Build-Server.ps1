param([string]$Go = 'go', [switch]$UpdatePVFDefault, [switch]$CheckSQL, [switch]$SkipEnvEnsure)
$ErrorActionPreference = 'Stop'

# 编译环境自举（业主 2026-10-05 指示：编译时自动触发，如果不存在整包就触发）。
#
# 背景：依赖缓存包 tools/tools-gopath-mod.zip 约 62.7 MB，单次推送会被远端断开
# （send-pack: unexpected disconnect），所以仓库里只存分片 tools/tools-gopath-mod.zip.partNN。
# 这里在编译前检查：整包不在、但分片在 → 先调 scripts/assemble-gopath-mod.ps1 拼回，
# 该脚本会按 tools/manifest.json 的 size/sha256 校验，幂等（已就绪则零写入）。
#
# -SkipEnvEnsure 只给测试/排查用：跳过这段，直接编译。
$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$PackPath = Join-Path $RepoRoot 'tools\tools-gopath-mod.zip'
$PartGlob = 'tools-gopath-mod.zip.part*'

if (-not $SkipEnvEnsure -and -not (Test-Path -LiteralPath $PackPath)) {
    $parts = @(Get-ChildItem -LiteralPath (Join-Path $RepoRoot 'tools') -File -Filter $PartGlob -ErrorAction SilentlyContinue)
    if ($parts.Count -gt 0) {
        Write-Output ("依赖缓存整包不存在，发现 {0} 个分片：先拼装…" -f $parts.Count)
        & (Join-Path $RepoRoot 'scripts\assemble-gopath-mod.ps1') -RepoRoot $RepoRoot
        if ($LASTEXITCODE -ne 0) { throw ("拼装 tools-gopath-mod.zip 失败（exit {0}）" -f $LASTEXITCODE) }
    } else {
        Write-Output '依赖缓存整包与分片都不在：跳过拼装（若编译失败，请按 tools/manifest.json 取 gopath-mod 包）。'
    }
}

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

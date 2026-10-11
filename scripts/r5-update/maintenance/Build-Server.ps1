param(
    [string]$Go = 'go', [switch]$UpdatePVFDefault, [switch]$CheckSQL,
    [string]$AcceptedCandidate, [string]$AcceptedSHA256,
    [string]$ExpectedDefaultSHA256, [string]$DefaultBackup, [switch]$SkipEnvEnsure,
    [string]$ReleaseRoot
)
$ErrorActionPreference = 'Stop'
# A bundled copy can publish into a player installation without developer scripts.
# This override is forbidden for builds or environment provisioning.
if ($ReleaseRoot -and (-not $AcceptedCandidate -or -not $UpdatePVFDefault -or -not $SkipEnvEnsure -or $CheckSQL)) {
    throw '-ReleaseRoot is only permitted for guarded publication with -SkipEnvEnsure.'
}

# The default launcher is the local edition. Shared cloud source is candidate-only.
if ($UpdatePVFDefault -and -not $AcceptedCandidate) {
    throw 'Local edition publication requires -AcceptedCandidate, both SHA256 guards and a new backup. Shared source builds cannot replace the local default.'
}

# 编译环境自举（业主 2026-10-05 指示：编译时自动触发，如果不存在整包就触发）。
#
# 背景：依赖缓存包 tools/tools-gopath-mod.zip 约 62.7 MB，单次推送会被远端断开
# （send-pack: unexpected disconnect），所以仓库里只存分片 tools/tools-gopath-mod.zip.partNN。
# 这里在编译前检查：整包不在、但分片在 → 先调 scripts/assemble-gopath-mod.ps1 拼回，
# 该脚本会按 tools/manifest.json 的 size/sha256 校验，幂等（已就绪则零写入）。
#
# -SkipEnvEnsure 只给测试/排查用：跳过这段，直接编译。
$RepoRoot = if ($ReleaseRoot) { (Resolve-Path -LiteralPath $ReleaseRoot).Path } else { (Resolve-Path (Join-Path $PSScriptRoot '..')).Path }
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

Push-Location (Join-Path $RepoRoot 'server/work/dfo-lan')
try {
    # Publish the exact user-tested binary without building unrelated live source.
    # All hash guards and the prior default backup are mandatory in this mode.
    if ($AcceptedCandidate) {
        if (-not $UpdatePVFDefault -or $CheckSQL -or
            $AcceptedSHA256 -notmatch '^[0-9a-fA-F]{64}$' -or
            $ExpectedDefaultSHA256 -notmatch '^[0-9a-fA-F]{64}$' -or -not $DefaultBackup) {
            throw 'Accepted publication requires -UpdatePVFDefault, both SHA256 guards and a new -DefaultBackup; do not combine with -CheckSQL.'
        }
        $candidate = (Resolve-Path -LiteralPath $AcceptedCandidate).Path
        $target = (Resolve-Path -LiteralPath 'bin/wireprobe-pvf.exe').Path
        if ((Get-FileHash -LiteralPath $candidate -Algorithm SHA256).Hash -ne $AcceptedSHA256) {
            throw 'Accepted candidate SHA256 mismatch; nothing published.'
        }
        # Go flag help uses stderr; collect it without PS 5.1 native-error promotion.
        $previousPreference = $ErrorActionPreference
        try {
            $ErrorActionPreference = 'Continue'
            $candidateHelp = (& $candidate -h 2>&1 | Out-String)
            $candidateHelpExit = $LASTEXITCODE
        } finally { $ErrorActionPreference = $previousPreference }
        if ($candidateHelpExit -ne 0) {
            throw 'Candidate help check failed; nothing published.'
        }
        if ($candidateHelp -match 'cloud-account-config') {
            throw 'Online edition candidate rejected: cloud account support must not enter the local default. Nothing published.'
        }
        if ((Get-FileHash -LiteralPath $target -Algorithm SHA256).Hash -ne $ExpectedDefaultSHA256) {
            throw 'Default changed since review; nothing published.'
        }
        $backup = [IO.Path]::GetFullPath($DefaultBackup)
        if (Test-Path -LiteralPath $backup) { throw 'Backup already exists; nothing published.' }
        if (Get-Process | Where-Object { $_.Path -eq $target }) {
            throw 'Default binary is running; close its session before publishing.'
        }
        Copy-Item -LiteralPath $target -Destination $backup
        if ((Get-FileHash -LiteralPath $backup -Algorithm SHA256).Hash -ne $ExpectedDefaultSHA256) {
            throw 'Default backup verification failed; nothing published.'
        }
        $temporary = $target + '.publish-' + [guid]::NewGuid().ToString('N')
        try {
            Copy-Item -LiteralPath $candidate -Destination $temporary
            if ((Get-FileHash -LiteralPath $temporary -Algorithm SHA256).Hash -ne $AcceptedSHA256) { throw 'Staged candidate SHA256 mismatch; nothing published.' }
            if ((Get-FileHash -LiteralPath $target -Algorithm SHA256).Hash -ne $ExpectedDefaultSHA256) { throw 'Default changed during publication; nothing published.' }
            [IO.File]::Replace($temporary, $target, [NullString]::Value)
            if ((Get-FileHash -LiteralPath $target -Algorithm SHA256).Hash -ne $AcceptedSHA256) {
                Copy-Item -LiteralPath $backup -Destination $target -Force
                throw 'Publication verification failed; previous default restored.'
            }
        } finally { if (Test-Path -LiteralPath $temporary) { Remove-Item -LiteralPath $temporary } }
        Write-Output 'Published the exact accepted gateway; shared source candidate and archived39 were preserved.'
        return
    }
    if ($AcceptedSHA256 -or $ExpectedDefaultSHA256 -or $DefaultBackup) {
        throw 'Publication guards require -AcceptedCandidate.'
    }
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
    Write-Output 'Built source candidate only; local default was preserved. Publish an accepted local candidate with the SHA256 guards and backup, or use a separate online deployment.'
} finally { Pop-Location }

# build-incremental.ps1 - 把指定提交（默认最后一次提交）中改动的文件打成增量更新包
#
# 用法（在仓库任意位置执行）：
#   powershell -NoProfile -ExecutionPolicy Bypass -File scripts\incremental-package\build-incremental.ps1
#   powershell -NoProfile -ExecutionPolicy Bypass -File scripts\incremental-package\build-incremental.ps1 -Commit HEAD~1
#   powershell -NoProfile -ExecutionPolicy Bypass -File scripts\incremental-package\build-incremental.ps1 -Commit <sha> -OutDir D:\out
#
# 产出：<OutDir>\DFO115US-增量更新-<提交短号>-<日期>.zip
#   解压到游戏根目录后，双击 安装增量更新.cmd（自动备份 + 更新），双击 还原上一版本.cmd（一键还原）。
#
# 设计要点：
#   * 只从 git 对象库导出（git archive），不读工作区，因此其它未提交改动不会混入更新包；
#   * 二进制文件（exe/dll/pvf 等）原样导出，不做文本转换；
#   * manifest.json 记录每个文件更新前后的 SHA256，目标机安装时据此预检与校验；
#   * 被删除的文件记为 D，安装脚本会先备份再删除，还原时从备份恢复。
[CmdletBinding()]
param(
    [string]$Commit = 'HEAD',
    [string]$OutDir = '',
    [string]$Name = '',
    [switch]$KeepStaging
)

$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.IO.Compression
Add-Type -AssemblyName System.IO.Compression.FileSystem

function Write-Step([string]$Text) { Write-Host ''; Write-Host ('== ' + $Text) -ForegroundColor Cyan }
function Write-Detail([string]$Text) { Write-Host ('   ' + $Text) }
function Write-Ok([string]$Text) { Write-Host ('   [OK] ' + $Text) -ForegroundColor Green }
function Write-Warn2([string]$Text) { Write-Host ('   [警告] ' + $Text) -ForegroundColor Yellow }

function Get-Sha256([string]$Path) {
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) { return $null }
    return (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()
}

function Invoke-GitCapture {
    # 通过 cmd 重定向读 git 输出：避免 PowerShell 5.1 按 ANSI 解码 UTF-8 造成中文乱码
    param([string]$RepoRoot, [string[]]$GitArgs)
    $tmp = Join-Path $env:TEMP ('df-inc-git-' + [guid]::NewGuid().ToString('n') + '.out')
    try {
        $quoted = ($GitArgs | ForEach-Object { '"' + ($_ -replace '"', '\"') + '"' }) -join ' '
        $line = 'git -C "' + $RepoRoot + '" ' + $quoted + ' > "' + $tmp + '" 2>nul'
        cmd /c $line | Out-Null
        $code = $LASTEXITCODE
        if (-not (Test-Path -LiteralPath $tmp)) { return [pscustomobject]@{ ExitCode = $code; Text = '' } }
        $text = [System.Text.Encoding]::UTF8.GetString([System.IO.File]::ReadAllBytes($tmp))
        return [pscustomobject]@{ ExitCode = $code; Text = $text }
    }
    finally { Remove-Item -LiteralPath $tmp -Force -ErrorAction SilentlyContinue }
}

function Export-GitChunk {
    # 用 git archive 把一组路径从提交里导出到目录（保持二进制原样）
    param([string]$RepoRoot, [string]$Commit, [string[]]$Paths, [string]$DestDir)
    $zip = Join-Path $env:TEMP ('df-inc-arc-' + [guid]::NewGuid().ToString('n') + '.zip')
    try {
        $gitArgs = @('-c', 'core.quotepath=false', 'archive', '--format=zip', '-o', $zip, $Commit, '--') + $Paths
        $quoted = ($gitArgs | ForEach-Object { '"' + ($_ -replace '"', '\"') + '"' }) -join ' '
        cmd /c ('git -C "' + $RepoRoot + '" ' + $quoted + ' >nul 2>nul') | Out-Null
        if ($LASTEXITCODE -ne 0) { return $false }
        if (-not (Test-Path -LiteralPath $zip -PathType Leaf)) { return $false }

        $archive = [System.IO.Compression.ZipFile]::OpenRead($zip)
        try {
            foreach ($entry in $archive.Entries) {
                if ([string]::IsNullOrEmpty($entry.Name)) { continue }
                $dest = Join-Path $DestDir ($entry.FullName -replace '/', '\')
                $parent = Split-Path -Parent $dest
                if (-not (Test-Path -LiteralPath $parent -PathType Container)) { New-Item -ItemType Directory -Path $parent -Force | Out-Null }
                [System.IO.Compression.ZipFileExtensions]::ExtractToFile($entry, $dest, $true)
            }
        }
        finally { $archive.Dispose() }
        return $true
    }
    finally { Remove-Item -LiteralPath $zip -Force -ErrorAction SilentlyContinue }
}

function Export-GitPaths {
    param([string]$RepoRoot, [string]$Commit, [string[]]$Paths, [string]$DestDir)
    $exported = @{}
    if (@($Paths).Count -eq 0) { return $exported }
    $chunkSize = 40
    for ($i = 0; $i -lt $Paths.Count; $i += $chunkSize) {
        $end = [Math]::Min($i + $chunkSize - 1, $Paths.Count - 1)
        $chunk = @($Paths[$i..$end])
        if (Export-GitChunk -RepoRoot $RepoRoot -Commit $Commit -Paths $chunk -DestDir $DestDir) {
            foreach ($p in $chunk) { $exported[$p] = $true }
            continue
        }
        Write-Warn2 ('批量导出失败，改为逐文件重试：' + ($chunk -join ', '))
        foreach ($p in $chunk) {
            $single = Export-GitChunk -RepoRoot $RepoRoot -Commit $Commit -Paths @($p) -DestDir $DestDir
            $exported[$p] = $single
        }
    }
    return $exported
}

# ---------------------------------------------------------------------------
# 0. 定位仓库与提交
# ---------------------------------------------------------------------------
$ScriptDir = $PSScriptRoot
$repoProbe = & git -C $ScriptDir rev-parse --show-toplevel 2>$null
if ($LASTEXITCODE -eq 0 -and $repoProbe) { $RepoRoot = ($repoProbe | Select-Object -First 1).Trim() }
else { $RepoRoot = Split-Path -Parent (Split-Path -Parent $ScriptDir) }
$RepoRoot = [System.IO.Path]::GetFullPath($RepoRoot).TrimEnd('\')

$shaRaw = & git -C $RepoRoot rev-parse ($Commit + '^{commit}') 2>$null
if ($LASTEXITCODE -ne 0 -or -not $shaRaw) { throw ('无效的提交：' + $Commit) }
$sha = ($shaRaw | Select-Object -First 1).Trim()
$shortRaw = & git -C $RepoRoot rev-parse --short $sha
$short = ($shortRaw | Select-Object -First 1).Trim()

# 逐项单独取，格式串只含一个 % 占位符，避免 cmd 误当作变量展开
$author = (Invoke-GitCapture -RepoRoot $RepoRoot -GitArgs @('log', '-1', '--format=%an', $sha)).Text.Trim()
$commitDate = (Invoke-GitCapture -RepoRoot $RepoRoot -GitArgs @('log', '-1', '--date=iso', '--format=%ad', $sha)).Text.Trim()
$subject = (Invoke-GitCapture -RepoRoot $RepoRoot -GitArgs @('log', '-1', '--format=%s', $sha)).Text.Trim()

$baseProbe = & git -C $RepoRoot rev-parse ($sha + '^') 2>$null
$base = $null
if ($LASTEXITCODE -eq 0 -and $baseProbe) { $base = ($baseProbe | Select-Object -First 1).Trim() }
$baseShort = ''
$baseSubject = ''
if ($base) {
    $bs = & git -C $RepoRoot rev-parse --short $base
    $baseShort = ($bs | Select-Object -First 1).Trim()
    $baseSubject = (Invoke-GitCapture -RepoRoot $RepoRoot -GitArgs @('log', '-1', '--format=%s', $base)).Text.Trim()
}

$stamp = Get-Date -Format 'yyyyMMdd'
if (-not $Name) { $Name = 'DFO115US-增量更新-' + $short + '-' + $stamp }
if (-not $OutDir) { $OutDir = Split-Path -Parent $RepoRoot }
$OutDir = [System.IO.Path]::GetFullPath($OutDir)
if (-not (Test-Path -LiteralPath $OutDir -PathType Container)) { New-Item -ItemType Directory -Path $OutDir -Force | Out-Null }
$zipPath = Join-Path $OutDir ($Name + '.zip')

Write-Step 'DFO 115us 增量更新包打包'
Write-Detail ('仓库：' + $RepoRoot)
Write-Detail ('提交：{0}  {1}' -f $short, $subject)
Write-Detail ('基线：{0}  {1}' -f $baseShort, $baseSubject)
Write-Detail ('输出：' + $zipPath)

# ---------------------------------------------------------------------------
# 1. 取本次提交的改动清单
# ---------------------------------------------------------------------------
Write-Step '步骤 1/6：解析提交改动清单'
$diffArgs = @('-c', 'core.quotepath=false', 'diff', '--name-status', '-z', '--no-renames')
if ($base) { $diffArgs += @($base, $sha) }
else { $diffArgs = @('-c', 'core.quotepath=false', 'diff-tree', '--root', '--no-commit-id', '-r', '--name-status', '-z', $sha) }
$diffRes = Invoke-GitCapture -RepoRoot $RepoRoot -GitArgs $diffArgs
if ($diffRes.ExitCode -ne 0) { throw '读取提交改动清单失败。' }

$tokens = $diffRes.Text.Split([char]0)
$changes = New-Object System.Collections.ArrayList
$i = 0
while ($i + 1 -lt $tokens.Count) {
    $st = $tokens[$i]
    $path = $tokens[$i + 1]
    $i += 2
    if ([string]::IsNullOrEmpty($st) -or [string]::IsNullOrEmpty($path)) { continue }
    $code = $st.Substring(0, 1).ToUpperInvariant()
    if ($code -eq 'T') { $code = 'M' }
    if ($code -eq 'R' -or $code -eq 'C') { $code = 'A' }
    [void]$changes.Add([pscustomobject]@{ status = $code; path = $path })
}
if ($changes.Count -eq 0) { throw ('提交 ' + $short + ' 没有任何改动，无需打包。') }

$countAdded = @($changes | Where-Object { $_.status -eq 'A' }).Count
$countModified = @($changes | Where-Object { $_.status -eq 'M' }).Count
$countDeleted = @($changes | Where-Object { $_.status -eq 'D' }).Count
Write-Detail ('新增 {0} 个 / 修改 {1} 个 / 删除 {2} 个' -f $countAdded, $countModified, $countDeleted)

# ---------------------------------------------------------------------------
# 2. 建暂存目录：payload（新版本文件）
# ---------------------------------------------------------------------------
Write-Step '步骤 2/6：从 git 对象库导出文件内容'
$stageRoot = Join-Path $env:TEMP ('df-inc-' + [guid]::NewGuid().ToString('n'))
$stagePkg = Join-Path $stageRoot $Name
$payloadDir = Join-Path $stagePkg 'payload'
$baseStage = Join-Path $stageRoot '_base'
New-Item -ItemType Directory -Path $payloadDir -Force | Out-Null

$newPaths = @($changes | Where-Object { $_.status -ne 'D' } | ForEach-Object { $_.path })
$oldPaths = @($changes | Where-Object { $_.status -ne 'A' } | ForEach-Object { $_.path })

$exportedNew = Export-GitPaths -RepoRoot $RepoRoot -Commit $sha -Paths $newPaths -DestDir $payloadDir
if ($base -and $oldPaths.Count -gt 0) {
    New-Item -ItemType Directory -Path $baseStage -Force | Out-Null
    $exportedOld = Export-GitPaths -RepoRoot $RepoRoot -Commit $base -Paths $oldPaths -DestDir $baseStage
}
else { $exportedOld = @{} }

$missing = @($newPaths | Where-Object { -not $exportedNew[$_] })
if ($missing.Count -gt 0) { throw ('以下文件无法从提交导出：' + ($missing -join ', ')) }
Write-Ok ('已导出 {0} 个文件内容。' -f $newPaths.Count)

# ---------------------------------------------------------------------------
# 3. 生成 manifest.json
# ---------------------------------------------------------------------------
Write-Step '步骤 3/6：生成 manifest.json（含逐文件 SHA256）'
$entries = New-Object System.Collections.ArrayList
foreach ($c in ($changes | Sort-Object path)) {
    $rel = $c.path
    $entry = [pscustomobject]@{
        path         = $rel
        status       = $c.status
        payload      = $null
        sizeAfter    = $null
        sha256After  = $null
        sha256Before = $null
    }

    if ($c.status -ne 'D') {
        $local = Join-Path $payloadDir ($rel -replace '/', '\')
        $entry.payload = 'payload/' + $rel
        $entry.sizeAfter = (Get-Item -LiteralPath $local).Length
        $entry.sha256After = Get-Sha256 $local
    }
    if ($c.status -ne 'A' -and $base) {
        $oldLocal = Join-Path $baseStage ($rel -replace '/', '\')
        if (Test-Path -LiteralPath $oldLocal -PathType Leaf) { $entry.sha256Before = Get-Sha256 $oldLocal }
    }
    [void]$entries.Add($entry)
}

$manifest = [pscustomobject]@{
    schema            = 1
    kind              = 'dfo115us-incremental-update'
    packageName       = $Name
    repo              = $RepoRoot
    commit            = $sha
    commitShort       = $short
    commitDate        = $commitDate
    commitAuthor      = $author
    commitSubject     = $subject
    baseCommit        = $base
    baseCommitShort   = $baseShort
    baseCommitSubject = $baseSubject
    createdAt         = (Get-Date).ToString('o')
    builder           = 'scripts/incremental-package/build-incremental.ps1'
    countAdded        = $countAdded
    countModified     = $countModified
    countDeleted      = $countDeleted
    fileCount         = $entries.Count
    entries           = @($entries)
}
$enc = New-Object System.Text.UTF8Encoding($false)
[System.IO.File]::WriteAllText((Join-Path $stagePkg 'manifest.json'), ($manifest | ConvertTo-Json -Depth 8), $enc)
Write-Ok 'manifest.json 已生成。'

# ---------------------------------------------------------------------------
# 4. 放入安装/还原脚本与说明
# ---------------------------------------------------------------------------
Write-Step '步骤 4/6：放入安装脚本、还原脚本与使用说明'
$tplDir = Join-Path $ScriptDir 'templates'
if (-not (Test-Path -LiteralPath $tplDir -PathType Container)) { throw ('缺少模板目录：' + $tplDir) }

$updateSrc = Join-Path $tplDir '_update'
$updateDst = Join-Path $stagePkg '_update'
New-Item -ItemType Directory -Path $updateDst -Force | Out-Null
foreach ($f in (Get-ChildItem -LiteralPath $updateSrc -File)) { Copy-Item -LiteralPath $f.FullName -Destination $updateDst -Force }
foreach ($f in (Get-ChildItem -LiteralPath $tplDir -File -Filter '*.cmd')) { Copy-Item -LiteralPath $f.FullName -Destination (Join-Path $stagePkg $f.Name) -Force }

$fileTableRows = @()
foreach ($e in ($changes | Sort-Object path)) {
    $label = '修改'
    if ($e.status -eq 'A') { $label = '新增' }
    elseif ($e.status -eq 'D') { $label = '删除' }
    $sizeText = '-'
    if ($e.status -ne 'D') {
        $local = Join-Path $payloadDir ($e.path -replace '/', '\')
        $sizeText = ('{0:N0} B' -f (Get-Item -LiteralPath $local).Length)
    }
    $fileTableRows += ('| {0} | `{1}` | {2} |' -f $label, $e.path, $sizeText)
}

$readme = (Get-Content -LiteralPath (Join-Path $tplDir '使用说明.md') -Raw -Encoding UTF8)
$tokensMap = [ordered]@{
    '{{PACKAGE_NAME}}'   = $Name
    '{{COMMIT_SHORT}}'   = $short
    '{{COMMIT}}'         = $sha
    '{{COMMIT_DATE}}'    = $commitDate
    '{{COMMIT_SUBJECT}}' = $subject
    '{{BASE_COMMIT}}'    = ($baseShort + ' ' + $baseSubject).Trim()
    '{{BUILD_TIME}}'     = (Get-Date).ToString('yyyy-MM-dd HH:mm:ss')
    '{{COUNTS}}'         = ('{0} 个（新增 {1} / 修改 {2} / 删除 {3}）' -f $entries.Count, $countAdded, $countModified, $countDeleted)
    '{{FILE_TABLE}}'     = (@('| 状态 | 文件 | 新版本大小 |', '| --- | --- | --- |') + $fileTableRows) -join "`r`n"
}
foreach ($k in $tokensMap.Keys) { $readme = $readme.Replace($k, [string]$tokensMap[$k]) }
[System.IO.File]::WriteAllText((Join-Path $stagePkg '使用说明.md'), $readme, (New-Object System.Text.UTF8Encoding($true)))
Write-Ok '安装脚本与使用说明已放入。'

# ---------------------------------------------------------------------------
# 5. 压缩（正斜杠条目名 + UTF-8 文件名 + 单层顶层目录）
# ---------------------------------------------------------------------------
Write-Step '步骤 5/6：压缩为 zip'
if (Test-Path -LiteralPath $zipPath) { Remove-Item -LiteralPath $zipPath -Force }
$zip = [System.IO.Compression.ZipFile]::Open($zipPath, [System.IO.Compression.ZipArchiveMode]::Create)
try {
    $srcFull = [System.IO.Path]::GetFullPath($stagePkg).TrimEnd('\')
    foreach ($f in (Get-ChildItem -LiteralPath $srcFull -Recurse -File -Force | Sort-Object FullName)) {
        $rel = $f.FullName.Substring($srcFull.Length + 1).Replace('\', '/')
        $entry = $zip.CreateEntry(($Name + '/' + $rel), [System.IO.Compression.CompressionLevel]::Optimal)
        $in = [System.IO.File]::OpenRead($f.FullName)
        try {
            $out = $entry.Open()
            try { $in.CopyTo($out) } finally { $out.Dispose() }
        }
        finally { $in.Dispose() }
    }
}
finally { $zip.Dispose() }
$zipMb = [Math]::Round((Get-Item -LiteralPath $zipPath).Length / 1MB, 2)
Write-Ok ('已生成：{0}（{1} MB）' -f $zipPath, $zipMb)

# ---------------------------------------------------------------------------
# 6. 校验包内容
# ---------------------------------------------------------------------------
Write-Step '步骤 6/6：校验更新包'
$verify = [System.IO.Compression.ZipFile]::OpenRead($zipPath)
try {
    $names = @($verify.Entries | ForEach-Object { $_.FullName })
    $need = @(
        ($Name + '/manifest.json'),
        ($Name + '/使用说明.md'),
        ($Name + '/安装增量更新.cmd'),
        ($Name + '/还原上一版本.cmd'),
        ($Name + '/查看备份记录.cmd'),
        ($Name + '/_update/update.ps1'),
        ($Name + '/_update/common.ps1')
    )
    $miss = @($need | Where-Object { $names -notcontains $_ })
    if ($miss.Count -gt 0) { throw ('更新包缺少文件：' + ($miss -join ', ')) }
    foreach ($e in @($entries)) {
        if ($e.status -eq 'D') { continue }
        if ($names -notcontains ($Name + '/' + $e.payload)) { throw ('更新包缺少载荷文件：' + $e.payload) }
    }
    Write-Ok ('校验通过：共 {0} 个条目。' -f $names.Count)
}
finally { $verify.Dispose() }

if (-not $KeepStaging) { Remove-Item -LiteralPath $stageRoot -Recurse -Force -ErrorAction SilentlyContinue }
else { Write-Detail ('暂存目录（未删除）：' + $stageRoot) }

Write-Host ''
Write-Host ('[成功] 增量更新包：' + $zipPath) -ForegroundColor Green
Write-Detail '把该 zip 解压到游戏根目录，进入解压出的文件夹，双击「安装增量更新.cmd」；双击「还原上一版本.cmd」即可一键还原。'

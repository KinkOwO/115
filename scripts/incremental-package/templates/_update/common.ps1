# common.ps1 - DFO 115us 增量更新包公共函数
# 兼容 Windows PowerShell 5.1 与 PowerShell 7+。
# 注意：本文件含中文，必须以 UTF-8 with BOM 保存，否则 5.1 会按 ANSI 解码而乱码。

function Write-Head([string]$Text) {
    Write-Host ''
    Write-Host ('== ' + $Text) -ForegroundColor Cyan
}

function Write-Info([string]$Text) { Write-Host ('   ' + $Text) }
function Write-Ok([string]$Text) { Write-Host ('   [OK] ' + $Text) -ForegroundColor Green }
function Write-Warn([string]$Text) { Write-Host ('   [警告] ' + $Text) -ForegroundColor Yellow }
function Write-Fail([string]$Text) { Write-Host ('   [错误] ' + $Text) -ForegroundColor Red }

function Get-FileSha256([string]$Path) {
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) { return $null }
    return (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()
}

function Get-NormalizedDir([string]$Path) {
    $full = [System.IO.Path]::GetFullPath($Path)
    return $full.TrimEnd([char]'\', [char]'/')
}

# 仓库相对路径（正斜杠）-> 目标机上的本地路径
function Get-TargetFilePath([string]$TargetDir, [string]$RelPath) {
    return (Join-Path $TargetDir ($RelPath -replace '/', '\'))
}

function Write-JsonFile([string]$Path, $Object) {
    $json = $Object | ConvertTo-Json -Depth 8
    $enc = New-Object System.Text.UTF8Encoding($false)
    [System.IO.File]::WriteAllText($Path, $json, $enc)
}

function Write-TextFile([string]$Path, [string]$Text) {
    $enc = New-Object System.Text.UTF8Encoding($true)
    [System.IO.File]::WriteAllText($Path, $Text, $enc)
}

function Read-Manifest([string]$PackageRoot) {
    $p = Join-Path $PackageRoot 'manifest.json'
    if (-not (Test-Path -LiteralPath $p -PathType Leaf)) { throw ('更新包缺少 manifest.json：' + $p) }
    return (Get-Content -LiteralPath $p -Raw -Encoding UTF8 | ConvertFrom-Json)
}

function Test-IsGameRoot([string]$Dir) {
    if (-not (Test-Path -LiteralPath $Dir -PathType Container)) { return $false }
    foreach ($m in @('scripts\启动游戏.cmd', 'scripts\启动服务端.cmd', 'server\work\dfo-lan', 'server\launcher.example.json')) {
        if (Test-Path -LiteralPath (Join-Path $Dir $m)) { return $true }
    }
    return $false
}

# 目标目录解析顺序：-Target 参数 > 包内 target.txt > 解压目录的上一级（默认）> 交互输入
function Resolve-TargetDir {
    param(
        [Parameter(Mandatory = $true)][string]$PackageRoot,
        [string]$Target = '',
        [switch]$Yes
    )
    if ($Target -and $Target.Trim()) {
        $t = Get-NormalizedDir ($Target.Trim().Trim('"'))
        if (-not (Test-Path -LiteralPath $t -PathType Container)) { throw ('目标目录不存在：' + $t) }
        return $t
    }

    $candidate = Get-NormalizedDir (Split-Path -Parent $PackageRoot)
    $targetFile = Join-Path $PackageRoot 'target.txt'
    if (Test-Path -LiteralPath $targetFile -PathType Leaf) {
        $line = Get-Content -LiteralPath $targetFile -Encoding UTF8 |
            Where-Object { $_ -and $_.Trim() -ne '' -and -not $_.Trim().StartsWith('#') } |
            Select-Object -First 1
        if ($line) { $candidate = Get-NormalizedDir ($line.Trim().Trim('"')) }
    }

    if ($Yes) { return $candidate }

    Write-Info ('默认目标目录：' + $candidate)
    if (Test-IsGameRoot $candidate) {
        Write-Info '该目录包含启动脚本/服务端目录，已判定为游戏根目录。'
    }
    else {
        Write-Warn '该目录下未发现 scripts\启动游戏.cmd / server\work\dfo-lan，请确认它是不是游戏根目录。'
    }

    $answer = ''
    try { $answer = Read-Host '回车使用默认目录，或直接输入游戏根目录的完整路径' }
    catch { $answer = '' }

    if ($answer -and $answer.Trim()) {
        $t = Get-NormalizedDir ($answer.Trim().Trim('"'))
        if (-not (Test-Path -LiteralPath $t -PathType Container)) { throw ('目标目录不存在：' + $t) }
        return $t
    }
    return $candidate
}

function Get-BackupRoot([string]$TargetDir) {
    return (Join-Path $TargetDir '_update-backup')
}

# 读取目标目录下全部备份记录（按时间倒序，最新在前）
function Get-BackupRecords([string]$TargetDir) {
    $list = @()
    $root = Get-BackupRoot $TargetDir
    if (-not (Test-Path -LiteralPath $root -PathType Container)) { return $list }
    foreach ($d in (Get-ChildItem -LiteralPath $root -Directory | Sort-Object Name -Descending)) {
        $j = Join-Path $d.FullName 'journal.json'
        if (Test-Path -LiteralPath $j -PathType Leaf) {
            $obj = $null
            try { $obj = Get-Content -LiteralPath $j -Raw -Encoding UTF8 | ConvertFrom-Json } catch { $obj = $null }
            if ($null -eq $obj) { continue }
            $list += [pscustomobject]@{
                Name       = $d.Name
                Dir        = $d.FullName
                Journal    = $obj
                Restored   = (Test-Path -LiteralPath (Join-Path $d.FullName 'restored.txt'))
                RolledBack = (Test-Path -LiteralPath (Join-Path $d.FullName 'rolled-back.txt'))
            }
        }
    }
    return $list
}

# 按 journal 还原：原先存在的文件从备份复制回去，原先不存在的文件删除
function Invoke-JournalRestore {
    param(
        [Parameter(Mandatory = $true)][string]$BackupDir,
        [Parameter(Mandatory = $true)][string]$TargetDir,
        [switch]$DryRun
    )
    $journalPath = Join-Path $BackupDir 'journal.json'
    if (-not (Test-Path -LiteralPath $journalPath -PathType Leaf)) { throw ('备份记录缺少 journal.json：' + $journalPath) }
    $journal = Get-Content -LiteralPath $journalPath -Raw -Encoding UTF8 | ConvertFrom-Json

    $restored = 0
    $removed = 0
    $failed = @()

    foreach ($e in @($journal.entries)) {
        $dst = Get-TargetFilePath $TargetDir $e.path
        if ($e.action -eq 'remove') {
            if (Test-Path -LiteralPath $dst -PathType Leaf) {
                if ($DryRun) { $removed++; continue }
                $item = Get-Item -LiteralPath $dst -Force
                if ($item.IsReadOnly) { $item.IsReadOnly = $false }
                Remove-Item -LiteralPath $dst -Force
                $removed++
            }
            continue
        }

        $src = Join-Path (Join-Path $BackupDir 'files') ($e.path -replace '/', '\')
        if (-not (Test-Path -LiteralPath $src -PathType Leaf)) {
            $failed += ($e.path + '（备份文件缺失）')
            continue
        }
        if ($DryRun) { $restored++; continue }

        $parent = Split-Path -Parent $dst
        if (-not (Test-Path -LiteralPath $parent -PathType Container)) { New-Item -ItemType Directory -Path $parent -Force | Out-Null }
        $existing = Get-Item -LiteralPath $dst -Force -ErrorAction SilentlyContinue
        if ($existing -and $existing.IsReadOnly) { $existing.IsReadOnly = $false }
        Copy-Item -LiteralPath $src -Destination $dst -Force
        if ($e.readOnlyBefore) { (Get-Item -LiteralPath $dst -Force).IsReadOnly = $true }

        $h = Get-FileSha256 $dst
        if ($e.sha256Before -and $h -ne $e.sha256Before) { $failed += ($e.path + '（还原后校验不一致）') }
        else { $restored++ }
    }

    return [pscustomobject]@{
        Restored = $restored
        Removed  = $removed
        Failed   = $failed
        Journal  = $journal
    }
}

function Format-BackupRecords($Records) {
    $i = 0
    foreach ($r in $Records) {
        $j = $r.Journal
        $cnt = @($j.entries).Count
        $flag = ''
        if ($r.Restored) { $flag = '（已还原）' }
        elseif ($r.RolledBack) { $flag = '（安装失败已自动回滚）' }
        Write-Host ('   [{0}] {1}  提交 {2}  共 {3} 个文件  {4}{5}' -f $i, $r.Name, $j.commitShort, $cnt, $j.commitSubject, $flag)
        $i++
    }
}

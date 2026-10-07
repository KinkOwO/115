# update.ps1 - DFO 115us 增量更新：安装 / 还原 / 查看记录
# 通常由包内 安装增量更新.cmd、还原上一版本.cmd 调用，也可手动执行：
#   powershell -NoProfile -ExecutionPolicy Bypass -File _update\update.ps1 -Mode Apply   -Target "C:\Game\dof\115us\115"
#   powershell -NoProfile -ExecutionPolicy Bypass -File _update\update.ps1 -Mode Restore -Target "C:\Game\dof\115us\115"
#   powershell -NoProfile -ExecutionPolicy Bypass -File _update\update.ps1 -Mode List    -Target "C:\Game\dof\115us\115"
[CmdletBinding()]
param(
    [ValidateSet('Apply', 'Restore', 'List')]
    [string]$Mode = 'Apply',
    [string]$Target = '',
    [string]$Backup = '',
    [switch]$Force,
    [switch]$Yes,
    [switch]$DryRun
)

$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'common.ps1')

$PackageRoot = Split-Path -Parent $PSScriptRoot

function Invoke-Apply {
    Write-Head 'DFO 115us 增量更新 - 安装'
    $manifest = Read-Manifest $PackageRoot
    $entries = @($manifest.entries)

    Write-Info ('更新包名：' + $manifest.packageName)
    Write-Info ('对应提交：{0}  {1}' -f $manifest.commitShort, $manifest.commitSubject)
    Write-Info ('打包时间：' + $manifest.createdAt)
    Write-Info ('文件数量：更新 {0} 个 / 新增 {1} 个 / 删除 {2} 个' -f $manifest.countModified, $manifest.countAdded, $manifest.countDeleted)

    $target = Resolve-TargetDir -PackageRoot $PackageRoot -Target $Target -Yes:$Yes
    Write-Info ('目标目录：' + $target)
    Write-Host ''

    # ---- 1. 预检：目标文件是否与基线一致 ----
    Write-Head '步骤 1/4：预检目标文件'
    $mismatch = @()
    foreach ($e in $entries) {
        $dst = Get-TargetFilePath $target $e.path
        $exists = Test-Path -LiteralPath $dst -PathType Leaf
        if ($e.status -eq 'D') {
            if ($exists) { Write-Info ('待删除：' + $e.path) }
            else { Write-Info ('待删除文件不存在，跳过：' + $e.path) }
            continue
        }
        if (-not $exists) {
            if ($e.status -eq 'A') { Write-Info ('新增：' + $e.path) }
            else {
                Write-Warn ('目标文件缺失，将直接写入：' + $e.path)
                $mismatch += $e.path
            }
            continue
        }
        $h = Get-FileSha256 $dst
        if ($e.sha256Before -and $h -ne $e.sha256Before) {
            Write-Warn ('目标文件与更新基线不一致（本机版本较新或已被手工修改）：' + $e.path)
            $mismatch += $e.path
        }
        elseif ($h -eq $e.sha256After) { Write-Info ('内容已是目标版本（仍会重新写入并备份）：' + $e.path) }
        else { Write-Info ('待更新：' + $e.path) }
    }

    if ($mismatch.Count -gt 0 -and -not $Force -and -not $Yes) {
        Write-Warn ('共有 {0} 个文件与更新基线不一致，继续安装会覆盖现有内容（原文件仍会先备份，可一键还原）。' -f $mismatch.Count)
        $a = Read-Host '是否继续？(y/N)'
        if ($a -notmatch '^(y|Y)') { throw '用户取消安装。' }
    }

    if ($DryRun) {
        Write-Host ''
        Write-Ok '干跑模式：未备份、未写入任何文件。'
        return
    }

    # ---- 2. 备份：把本次会被改动/删除的文件原样备份到 <目标>\_update-backup\<时间戳>_<提交> ----
    Write-Head '步骤 2/4：备份原文件'
    $stamp = (Get-Date -Format 'yyyyMMdd-HHmmss') + '_' + $manifest.commitShort
    $backupDir = Join-Path (Get-BackupRoot $target) $stamp
    if (Test-Path -LiteralPath $backupDir) { Remove-Item -LiteralPath $backupDir -Recurse -Force }
    New-Item -ItemType Directory -Path $backupDir -Force | Out-Null
    $filesDir = Join-Path $backupDir 'files'

    $journalEntries = New-Object System.Collections.ArrayList
    foreach ($e in $entries) {
        $dst = Get-TargetFilePath $target $e.path
        if (-not (Test-Path -LiteralPath $dst -PathType Leaf)) {
            [void]$journalEntries.Add([pscustomobject]@{
                    path           = $e.path
                    action         = 'remove'   # 更新前不存在 -> 还原时删除
                    sha256Before   = $null
                    backupFile     = $null
                    readOnlyBefore = $false
                })
            continue
        }

        $bk = Join-Path $filesDir ($e.path -replace '/', '\')
        $parent = Split-Path -Parent $bk
        if (-not (Test-Path -LiteralPath $parent -PathType Container)) { New-Item -ItemType Directory -Path $parent -Force | Out-Null }
        Copy-Item -LiteralPath $dst -Destination $bk -Force
        $item = Get-Item -LiteralPath $dst -Force
        [void]$journalEntries.Add([pscustomobject]@{
                path           = $e.path
                action         = 'restore'  # 更新前存在 -> 还原时复制回去
                sha256Before   = (Get-FileSha256 $dst)
                backupFile     = ('files/' + $e.path)
                readOnlyBefore = [bool]$item.IsReadOnly
            })
    }

    # 备份与安装分开落盘：先写 journal，即使安装中途断电也能一键还原
    $journal = [pscustomobject]@{
        schema        = 1
        kind          = 'dfo115us-incremental-update'
        packageName   = $manifest.packageName
        commit        = $manifest.commit
        commitShort   = $manifest.commitShort
        commitSubject = $manifest.commitSubject
        target        = $target
        startedAt     = (Get-Date).ToString('o')
        appliedAt     = $null
        entries       = @($journalEntries)
    }
    Write-JsonFile (Join-Path $backupDir 'journal.json') $journal
    Write-Ok ('备份完成：' + $backupDir)
    Write-Info ('共备份 {0} 个文件。' -f @($journalEntries | Where-Object { $_.action -eq 'restore' }).Count)

    # ---- 3. 应用增量文件（任何一步失败自动回滚） ----
    Write-Head '步骤 3/4：写入增量文件'
    $applied = 0
    $added = 0
    $deleted = 0
    try {
        foreach ($e in $entries) {
            $dst = Get-TargetFilePath $target $e.path
            if ($e.status -eq 'D') {
                if (Test-Path -LiteralPath $dst -PathType Leaf) {
                    $item = Get-Item -LiteralPath $dst -Force
                    if ($item.IsReadOnly) { $item.IsReadOnly = $false }
                    Remove-Item -LiteralPath $dst -Force
                    $deleted++
                    Write-Ok ('已删除 ' + $e.path)
                }
                continue
            }

            $src = Join-Path $PackageRoot ($e.payload -replace '/', '\')
            if (-not (Test-Path -LiteralPath $src -PathType Leaf)) { throw ('更新包缺少文件：' + $e.payload) }
            $parent = Split-Path -Parent $dst
            if (-not (Test-Path -LiteralPath $parent -PathType Container)) { New-Item -ItemType Directory -Path $parent -Force | Out-Null }
            $existing = Get-Item -LiteralPath $dst -Force -ErrorAction SilentlyContinue
            if ($existing -and $existing.IsReadOnly) { $existing.IsReadOnly = $false }
            Copy-Item -LiteralPath $src -Destination $dst -Force

            $h = Get-FileSha256 $dst
            if ($h -ne $e.sha256After) { throw ('写入后校验失败：' + $e.path) }
            if ($e.status -eq 'A') { $added++; Write-Ok ('已新增 ' + $e.path) }
            else { $applied++; Write-Ok ('已更新 ' + $e.path) }
        }
    }
    catch {
        Write-Fail $_.Exception.Message
        Write-Warn '安装未完成，正在用本次备份自动回滚...'
        $r = Invoke-JournalRestore -BackupDir $backupDir -TargetDir $target
        Write-Info ('已回滚 {0} 个文件，移除 {1} 个新增文件。' -f $r.Restored, $r.Removed)
        if (@($r.Failed).Count -gt 0) { Write-Fail ('回滚未完成：' + (@($r.Failed) -join '; ')) }
        else { Write-TextFile (Join-Path $backupDir 'rolled-back.txt') ((Get-Date).ToString('o')) }
        throw '安装失败，已回滚到安装前状态。'
    }

    # ---- 4. 记录结果 ----
    Write-Head '步骤 4/4：记录结果'
    $journal.appliedAt = (Get-Date).ToString('o')
    Write-JsonFile (Join-Path $backupDir 'journal.json') $journal

    $result = [pscustomobject]@{
        appliedAt     = $journal.appliedAt
        target        = $target
        packageName   = $manifest.packageName
        commit        = $manifest.commit
        commitShort   = $manifest.commitShort
        commitSubject = $manifest.commitSubject
        applied       = $applied
        added         = $added
        deleted       = $deleted
        backupDir     = $backupDir
    }
    Write-JsonFile (Join-Path $backupDir 'result.json') $result
    Write-JsonFile (Join-Path (Get-BackupRoot $target) 'last.json') $result

    Write-Host ''
    Write-Ok ('安装完成：新增 {0} 个，更新 {1} 个，删除 {2} 个。' -f $added, $applied, $deleted)
    Write-Info ('备份目录：' + $backupDir)
    Write-Info '如需还原，双击包内「还原上一版本.cmd」，或执行：'
    Write-Info ('  powershell -NoProfile -ExecutionPolicy Bypass -File "_update\update.ps1" -Mode Restore -Target "' + $target + '"')
}

function Invoke-Restore {
    Write-Head 'DFO 115us 增量更新 - 一键还原'
    $target = Resolve-TargetDir -PackageRoot $PackageRoot -Target $Target -Yes:$Yes
    Write-Info ('目标目录：' + $target)

    $records = @(Get-BackupRecords $target)
    if ($records.Count -eq 0) {
        Write-Warn ('未找到任何备份记录：' + (Get-BackupRoot $target))
        Write-Info '说明：备份是安装增量更新时自动生成的，未安装过更新就无需还原。'
        return
    }

    Write-Host ''
    Write-Info '可用备份记录（新 -> 旧）：'
    Format-BackupRecords $records

    $chosen = $null
    if ($Backup -and $Backup.Trim()) {
        $key = $Backup.Trim().Trim('"')
        $chosen = $records | Where-Object { $_.Name -eq $key -or $_.Dir -eq $key } | Select-Object -First 1
        if (-not $chosen) { throw ('未找到指定备份：' + $key) }
    }
    else {
        $chosen = $records | Where-Object { -not $_.Restored -and -not $_.RolledBack } | Select-Object -First 1
        if (-not $chosen) {
            $chosen = $records[0]
            Write-Warn '全部备份都已还原过（或为失败已回滚的记录），默认仍选择最新一条。'
        }
    }

    Write-Host ''
    Write-Info ('将还原到：{0}（{1}）' -f $chosen.Name, $chosen.Journal.commitSubject)
    $preview = Invoke-JournalRestore -BackupDir $chosen.Dir -TargetDir $target -DryRun
    Write-Info ('涉及文件 {0} 个：回写 {1} 个、删除更新新增的 {2} 个。' -f @($chosen.Journal.entries).Count, $preview.Restored, $preview.Removed)

    if ($DryRun) {
        Write-Host ''
        Write-Ok '干跑模式：未改动任何文件。'
        return
    }
    if (-not $Yes) {
        $a = Read-Host '确认还原？(y/N)'
        if ($a -notmatch '^(y|Y)') { throw '用户取消还原。' }
    }

    $r = Invoke-JournalRestore -BackupDir $chosen.Dir -TargetDir $target
    Write-Host ''
    Write-Ok ('已回写 {0} 个文件，删除更新新增的 {1} 个文件。' -f $r.Restored, $r.Removed)
    if (@($r.Failed).Count -gt 0) {
        Write-Fail ('以下文件还原失败：' + (@($r.Failed) -join '; '))
        throw '还原未完全成功，请查看上方错误。'
    }
    Write-TextFile (Join-Path $chosen.Dir 'restored.txt') ((Get-Date).ToString('o'))
    Write-Info '还原完成，已恢复到安装该更新之前的状态。'
}

function Invoke-List {
    Write-Head 'DFO 115us 增量更新 - 备份记录'
    $target = Resolve-TargetDir -PackageRoot $PackageRoot -Target $Target -Yes:$Yes
    Write-Info ('目标目录：' + $target)
    $records = @(Get-BackupRecords $target)
    if ($records.Count -eq 0) { Write-Warn ('未找到任何备份记录：' + (Get-BackupRoot $target)); return }
    Write-Host ''
    Format-BackupRecords $records
}

try {
    switch ($Mode) {
        'Apply' { Invoke-Apply }
        'Restore' { Invoke-Restore }
        'List' { Invoke-List }
    }
    exit 0
}
catch {
    Write-Host ''
    Write-Fail $_.Exception.Message
    exit 1
}

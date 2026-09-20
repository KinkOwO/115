<#
生成交付包清单：package-manifest.json 与 MANIFEST.sha256。

包根 = 本脚本所在目录（server\），清单里的路径相对包根、用 / 分隔。
（与历史清单一致：清单字段含 created_at/purpose/... 元数据 + files[] 的 path/bytes/sha256。）

排除（与历史清单一致）：
  runtime\**                        运行时会话、日志、pgdata
  MANIFEST.sha256 / package-manifest.json   清单自身（自引用会让哈希永远对不上）
  launcher.local.json               本机私有配置（未入库）
  configs\ 下由外部生成的派生大资源   见 $excludedConfigs
保护：超过 -BigFileThresholdMB 的文件默认排除并打印 WARNING —— 将来新增派生资源不会
悄悄进清单，必须显式决定。

用法：
  powershell -ExecutionPolicy Bypass -File server\Build-Manifest.ps1
  powershell -ExecutionPolicy Bypass -File server\Build-Manifest.ps1 -WhatIfOnly   # 只报告差异，不写文件
#>
param(
    [string]$Root = $PSScriptRoot,
    [int]$BigFileThresholdMB = 50,
    [switch]$WhatIfOnly
)
$ErrorActionPreference = 'Stop'
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8

$excludedDirs = @('runtime')
$excludedFiles = @('MANIFEST.sha256', 'package-manifest.json', 'launcher.local.json')
$excludedConfigs = @(
    'booster-catalog.json', 'shop-vault-release.json', 'shop-special-candidate.json',
    'dungeons.full.json', 'items.index.json',
    'equipment-full.data', 'equipment-full.index.json'
)

$rootFull = (Resolve-Path $Root).Path
$relOf = { param($full) $full.Substring($rootFull.Length + 1) -replace '\\', '/' }

$entries = New-Object System.Collections.Generic.List[object]
$skipped = New-Object System.Collections.Generic.List[string]
foreach ($f in (Get-ChildItem -Path $rootFull -Recurse -File | Sort-Object { & $relOf $_.FullName })) {
    $rel = & $relOf $f.FullName
    $parts = $rel -split '/'
    if ($parts -contains 'runtime') { continue }
    if ($parts -contains '__pycache__' -or $rel -like '*.pyc') { continue }
    if ($excludedFiles -contains $rel) { continue }
    if ($rel -like 'work/dfo-lan/configs/*' -and $excludedConfigs -contains (Split-Path $rel -Leaf)) { continue }
    if ($f.Length -gt $BigFileThresholdMB * 1MB) {
        Write-Warning ("排除 {0}（{1:N1} MB 超过 {2} MB 阈值）：确实要进清单请在脚本里显式放行" -f $rel, ($f.Length / 1MB), $BigFileThresholdMB)
        $skipped.Add($rel) | Out-Null
        continue
    }
    try {
        # 与历史清单一致：文本文件按 LF 归一化后计算哈希（发布走 git 导出，包内是 LF），
        # 二进制（含 NUL）按原始字节。行尾差异因此不会污染清单。
        $raw = [System.IO.File]::ReadAllBytes($f.FullName)
        if ([Array]::IndexOf($raw, [byte]0) -ge 0) {
            $payload = $raw
        }
        else {
            $payload = [System.Text.Encoding]::UTF8.GetBytes(
                ([System.Text.Encoding]::UTF8.GetString($raw)).Replace("`r`n", "`n"))
        }
        $sha = [System.Security.Cryptography.SHA256]::Create().ComputeHash($payload)
        $hash = -join ($sha | ForEach-Object { $_.ToString('x2') })
        $size = $payload.Length
    }
    catch {
        Write-Warning ("跳过不可读文件 {0}：{1}" -f $rel, $_.Exception.Message)
        $skipped.Add($rel) | Out-Null
        continue
    }
    $entries.Add([pscustomobject]@{ path = $rel; bytes = $size; sha256 = $hash }) | Out-Null
}

$manifestPath = Join-Path $rootFull 'package-manifest.json'
$shaPath = Join-Path $rootFull 'MANIFEST.sha256'
$meta = [ordered]@{
    created_at                   = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ss.ffffff") + "+00:00"
    purpose                      = 'server source and portable Windows launcher developer handoff'
    default_runtime              = 'archived39'
    source_build                 = 'restored-source-candidate-not-real-client-accepted'
    full_client_included         = $false
    player_database_included     = $false
    private_credentials_included = $false
}
if (Test-Path $manifestPath) {
    try {
        $old = Get-Content $manifestPath -Raw -Encoding UTF8 | ConvertFrom-Json
        foreach ($k in @('purpose', 'default_runtime', 'source_build', 'full_client_included', 'player_database_included', 'private_credentials_included')) {
            if ($null -ne $old.$k) { $meta[$k] = $old.$k }
        }
        if ($WhatIfOnly) {
            $oldMap = @{}
            foreach ($o in $old.files) { $oldMap[$o.path] = $o }
            $newMap = @{}
            foreach ($n in $entries) { $newMap[$n.path] = $n }
            $added = $newMap.Keys | Where-Object { -not $oldMap.ContainsKey($_) } | Sort-Object
            $gone = $oldMap.Keys | Where-Object { -not $newMap.ContainsKey($_) } | Sort-Object
            $changed = $newMap.Keys | Where-Object {
                $oldMap.ContainsKey($_) -and ($oldMap[$_].bytes -ne $newMap[$_].bytes -or $oldMap[$_].sha256 -ne $newMap[$_].sha256)
            } | Sort-Object
            Write-Host ("`n旧清单 {0} 项 / 新清单 {1} 项" -f $old.files.Count, $entries.Count)
            Write-Host ("新增 {0}：" -f $added.Count); $added | ForEach-Object { Write-Host "  + $_" }
            Write-Host ("丢失 {0}：" -f $gone.Count); $gone | ForEach-Object { Write-Host "  - $_" }
            Write-Host ("变更 {0}：" -f $changed.Count); $changed | ForEach-Object { Write-Host ("  ~ {0}  {1} -> {2} bytes" -f $_, $oldMap[$_].bytes, $newMap[$_].bytes) }
            return
        }
    }
    catch {
        Write-Warning ("旧清单不可解析，按新元数据生成：{0}" -f $_.Exception.Message)
    }
}

$sb = New-Object System.Text.StringBuilder
[void]$sb.AppendLine('{')
foreach ($k in $meta.Keys) {
    $v = $meta[$k]
    $json = if ($v -is [bool]) { if ($v) { 'true' } else { 'false' } } else { ConvertTo-Json $v -Compress }
    [void]$sb.AppendLine(('  "{0}": {1},' -f $k, $json))
}
[void]$sb.AppendLine('  "files": [')
for ($i = 0; $i -lt $entries.Count; $i++) {
    $e = $entries[$i]
    $comma = if ($i -lt $entries.Count - 1) { ',' } else { '' }
    [void]$sb.AppendLine('    {')
    [void]$sb.AppendLine(('      "path": {0},' -f (ConvertTo-Json $e.path -Compress)))
    [void]$sb.AppendLine(('      "bytes": {0},' -f $e.bytes))
    [void]$sb.AppendLine(('      "sha256": "{0}"' -f $e.sha256))
    [void]$sb.AppendLine(('    }' + $comma))
}
[void]$sb.AppendLine('  ]')
[void]$sb.AppendLine('}')

$utf8 = New-Object System.Text.UTF8Encoding($false)
[System.IO.File]::WriteAllText($manifestPath, $sb.ToString(), $utf8)
$lines = $entries | ForEach-Object { '{0}  {1}' -f $_.sha256, $_.path }
[System.IO.File]::WriteAllText($shaPath, (($lines -join "`r`n") + "`r`n"), $utf8)

Write-Host ("已写出 {0} 项（排除 {1} 项超阈值文件）" -f $entries.Count, $skipped.Count)
Write-Host ("  {0}" -f $manifestPath)
Write-Host ("  {0}" -f $shaPath)

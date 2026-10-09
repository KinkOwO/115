# DFO 115us 环境自举（纯 PowerShell，按 tools\manifest.json 解包自带运行时）
#
# 背景（2026-10-07）：`scripts\configure_env.py` 在「去 Python」那一轮被删除，
# 但 `scripts\配置环境.cmd` 一直还调它 —— 别人 clone 下来双击必挂。本脚本是它的
# 纯 PowerShell 替代：**不再需要 Python**，数据源就是仓库自带的 `tools\manifest.json`。
#
# 用法：
#   powershell -NoProfile -ExecutionPolicy Bypass -File scripts\configure-env.ps1
#   powershell -NoProfile -ExecutionPolicy Bypass -File scripts\configure-env.ps1 -Check
#   powershell -NoProfile -ExecutionPolicy Bypass -File scripts\configure-env.ps1 -Package go,gopath-mod
#   powershell -NoProfile -ExecutionPolicy Bypass -File scripts\configure-env.ps1 -Force
#
# 做什么：按清单 `packages[]` 把仓库自带的 zip 解到各自的 `target`：
#   go             → tools                 （Go 1.26 工具链 → tools\go\bin\go.exe）
#   gopath-mod     → tools                 （模块缓存 → tools\gopath\pkg\mod；整包不在时先拼分片）
#   server-src     → server\work\dfo-lan   （服务端源码树）
#   server-configs → server\work\dfo-lan   （服务端运行配置）
#   server-bin     → server                （预编译 wireprobe-pvf.exe / dfolauncher.exe）
#
# 幂等：清单条目的 `check` 路径已存在就跳过，只有 `-Force` 才重解。
# 校验：解包前按清单的 `size` + `sha256` 核对整包；不一致就报错，绝不解半个包。
#
# git 工作区保护（2026-10-07 沙箱实测后定，别删这段）：
#   包里装的是**发布快照**，git 工作区里是**源码**，两边不一致时解包就是给跟踪文件降级。
#   实测：`tools\tools-server-bin.zip` 里那份 `cmd\wireprobe\testdata\config_help.json` 比工作区旧
#   （还留着上游已经删掉的 `-boostup-challenge`），解包后 `cmd\wireprobe` 的
#   `TestWireprobeConfigHelpAndCLIRejection` 直接 FAIL。因此：
#     1) git 工作区里，目标落在服务端源码树（`server`、`server\work\dfo-lan`）的包**默认一律不解**；
#     2) 其余包解包前再做一道机械校验：包内文件与工作区里的**跟踪文件**同名且内容不一致 → 跳过；
#     3) 解包目录（没有 `.git`，如玩家的整合包）不受这些限制，包内容照解 —— 那才是它的用途。
#   `-Force` 是唯一的覆盖开关（会按包内快照覆盖上面那些跟踪文件，请自己确认代价）。
#
# 退出码：0 全部就绪/补齐（含按上面规则跳过的包，跳过会在输出里点名）；
#         2 有包既没就绪、也没有可用的 zip（需要人工取包）；
#         3 整包校验不一致或解包失败；1 脚本自身错误。
# 只读性：`-Check` 不写任何文件；正常模式只写各包 `target` 目录，
#         以及 gopath-mod 整包缺失时由 assemble-gopath-mod.ps1 拼出的 `tools\tools-gopath-mod.zip`。

param(
    [string]$RepoRoot = '',
    [string[]]$Package = @(),
    [switch]$Check,
    [switch]$Force
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

function Get-ManifestProp($Entry, [string]$Name) {
    $prop = $Entry.PSObject.Properties[$Name]
    if ($null -eq $prop -or $null -eq $prop.Value) { return '' }
    return [string]$prop.Value
}

function Get-Sha256([string]$Path) {
    return (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLower()
}

# 包内某个条目与工作区同名文件内容是否一致（尺寸不同直接判不同；同尺寸比 SHA256）。
function Test-ZipEntryEqualsFile($Entry, [string]$Path) {
    $file = Get-Item -LiteralPath $Path
    if ($file.Length -ne $Entry.Length) { return $false }
    $sha = [System.Security.Cryptography.SHA256]::Create()
    try {
        $stream = $Entry.Open()
        try { $entryHash = [BitConverter]::ToString($sha.ComputeHash($stream)) } finally { $stream.Close() }
    }
    finally { $sha.Dispose() }
    $fileHash = (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash
    return (($entryHash -replace '-', '') -eq $fileHash)
}

# 包内会覆盖「工作区里内容不同的跟踪文件」的条目清单（机械校验，返回相对路径数组）。
function Get-ZipTrackedConflicts([string]$ZipPath, [string]$TargetRel, $TrackedSet) {
    $conflicts = @()
    if ($null -eq $TrackedSet) { return @($conflicts) }
    $prefix = ($TargetRel -replace '\\', '/').TrimEnd('/')
    $zip = [System.IO.Compression.ZipFile]::OpenRead($ZipPath)
    try {
        foreach ($entry in $zip.Entries) {
            if ([string]::IsNullOrEmpty($entry.Name)) { continue }   # 目录项
            $rel = $entry.FullName -replace '\\', '/'
            if ($prefix) { $rel = "$prefix/$rel" }
            if (-not $TrackedSet.Contains($rel)) { continue }
            $abs = Join-Path $RepoRoot ($rel -replace '/', '\')
            if (-not (Test-Path -LiteralPath $abs -PathType Leaf)) { continue }
            if (-not (Test-ZipEntryEqualsFile $entry $abs)) { $conflicts += $rel }
        }
    }
    finally { $zip.Dispose() }
    return @($conflicts)
}

try {
    if ([string]::IsNullOrWhiteSpace($RepoRoot)) {
        # 本脚本位于 <repo>/scripts/，仓库根 = 上一层。
        $RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
    }
    $manifestPath = Join-Path $RepoRoot 'tools\manifest.json'
    if (-not (Test-Path -LiteralPath $manifestPath)) {
        throw ("找不到清单：{0}（tools\ 没落地？先确认解包目录完整）" -f $manifestPath)
    }
    $doc = Get-Content -LiteralPath $manifestPath -Raw -Encoding UTF8 | ConvertFrom-Json
    $all = @($doc.packages)
    if ($all.Count -eq 0) { throw ("清单里没有 packages[]：{0}" -f $manifestPath) }

    if ($Package.Count -gt 0) {
        $known = @($all | ForEach-Object { Get-ManifestProp $_ 'name' })
        foreach ($want in $Package) {
            if ($known -notcontains $want) {
                throw ("清单里没有包 '{0}'（有：{1}）" -f $want, ($known -join ', '))
            }
        }
        $all = @($all | Where-Object { $Package -contains (Get-ManifestProp $_ 'name') })
    }

    # git 工作区判定 + 跟踪文件集合（用于保护层 1/2）。
    $hasGit = (Test-Path -LiteralPath (Join-Path $RepoRoot '.git')) -and ($null -ne (Get-Command git -ErrorAction SilentlyContinue))
    $trackedSet = $null
    if ($hasGit) {
        $trackedRaw = @(& git -C $RepoRoot -c core.quotePath=false ls-files 2>$null)
        if ($LASTEXITCODE -eq 0) {
            $trackedSet = New-Object 'System.Collections.Generic.HashSet[string]' ([System.StringComparer]::OrdinalIgnoreCase)
            foreach ($t in $trackedRaw) {
                if ([string]::IsNullOrWhiteSpace($t)) { continue }
                [void]$trackedSet.Add(($t -replace '\\', '/'))
            }
        }
    }
    # 目标落在服务端源码树里的包：git 工作区里默认不解（保护层 1）。
    $sourceTreeTargets = @('server', 'server/work/dfo-lan')

    Write-Host '================================================================' -ForegroundColor Cyan
    Write-Host ' DFO 115us 环境自举（tools\manifest.json 驱动，不需要 Python）' -ForegroundColor Cyan
    Write-Host '================================================================' -ForegroundColor Cyan
    Write-Host (" 仓库根：{0}" -f $RepoRoot)
    Write-Host (" 模式：{0}" -f $(if ($Check) { '只读体检（-Check，不写盘）' } else { '补齐缺失的包' }))
    Write-Host (" git 工作区：{0}" -f $(if ($hasGit) { '是（跟踪文件受保护，见脚本头部说明）' } else { '否（解包目录，包内容照解）' }))
    Write-Host ''

    $failed = 0
    $pending = 0
    $skipped = 0
    $rows = @()

    # 取可用的整包：存在且 size+sha256 对齐 → 直接用；否则有分片时先拼（-Check 只报告）。
    function Resolve-PackageZip([string]$Url, [long]$WantSize, [string]$WantSha) {
        $zip = Join-Path $RepoRoot $Url
        if (Test-Path -LiteralPath $zip) {
            $haveSize = (Get-Item -LiteralPath $zip).Length
            if ($haveSize -eq $WantSize -and (Get-Sha256 $zip) -eq $WantSha) { return $zip }
            Write-Host ("   [提示] {0} 与清单不一致（size={1}，期望 {2}）" -f (Split-Path $zip -Leaf), $haveSize, $WantSize) -ForegroundColor Yellow
        }
        $dir = Split-Path $zip -Parent
        $parts = @(Get-ChildItem -LiteralPath $dir -File -ErrorAction SilentlyContinue |
            Where-Object { $_.Name -like ((Split-Path $zip -Leaf) + '.part*') })
        if ($parts.Count -eq 0) { return '' }
        if ($Check) {
            Write-Host ("   [待拼] 发现 {0} 个分片（-Check 不拼；去掉 -Check 会先跑 scripts\assemble-gopath-mod.ps1）" -f $parts.Count) -ForegroundColor Yellow
            return ''
        }
        Write-Host ("   [拼装] 调用 scripts\assemble-gopath-mod.ps1（{0} 个分片）" -f $parts.Count)
        & (Join-Path $PSScriptRoot 'assemble-gopath-mod.ps1') -RepoRoot $RepoRoot | ForEach-Object { Write-Host ("   {0}" -f $_) }
        if ($LASTEXITCODE -ne 0) {
            Write-Host ("   [失败] 拼装退出码 {0}" -f $LASTEXITCODE) -ForegroundColor Red
            return ''
        }
        if (-not (Test-Path -LiteralPath $zip)) { return '' }
        if ((Get-Item -LiteralPath $zip).Length -ne $WantSize) { return '' }
        if ((Get-Sha256 $zip) -ne $WantSha) { return '' }
        return $zip
    }

    foreach ($entry in $all) {
        $name = Get-ManifestProp $entry 'name'
        $checkRel = Get-ManifestProp $entry 'check'
        $targetRel = Get-ManifestProp $entry 'target'
        $url = Get-ManifestProp $entry 'url'
        $note = Get-ManifestProp $entry 'note'
        $wantSize = [long](Get-ManifestProp $entry 'size')
        $wantSha = (Get-ManifestProp $entry 'sha256').ToLower()
        $checkPath = Join-Path $RepoRoot $checkRel
        $targetPath = Join-Path $RepoRoot $targetRel
        $targetNorm = ($targetRel -replace '\\', '/').TrimEnd('/')

        Write-Host ("== {0} ==" -f $name) -ForegroundColor Cyan
        if ($note) { Write-Host ("   {0}" -f $note) -ForegroundColor DarkGray }

        if ((Test-Path -LiteralPath $checkPath) -and -not $Force) {
            Write-Host ("   [就绪] {0}" -f $checkRel) -ForegroundColor Green
            $rows += [pscustomobject]@{ 包 = $name; 结果 = '就绪'; 位置 = $checkRel }
            continue
        }

        # 保护层 1：目标落在服务端源码树 → git 工作区里默认不解。
        if ($hasGit -and ($sourceTreeTargets -contains $targetNorm) -and -not $Force) {
            Write-Host ("   [跳过] {0} 不在，但本包目标是服务端源码树 {1}，而这是 git 工作区。" -f $checkRel, $targetRel) -ForegroundColor Yellow
            Write-Host  '          包内是发布快照，工作区是源码：解包会把跟踪文件降级回旧快照（实测 server-bin' -ForegroundColor Yellow
            Write-Host  '          的 testdata 比工作区旧，解包后 cmd/wireprobe 的那个单测直接 FAIL）。' -ForegroundColor Yellow
            Write-Host  '          要按包内快照覆盖请显式加 -Force；只想编译服务端的话，装好 go + gopath-mod 就够了。' -ForegroundColor Yellow
            $skipped++
            $rows += [pscustomobject]@{ 包 = $name; 结果 = '跳过(源码树)'; 位置 = $targetRel }
            continue
        }

        $zip = Resolve-PackageZip $url $wantSize $wantSha
        if ($zip -eq '') {
            Write-Host ("   [缺包] {0} 不可用：清单里的 zip 与分片都没找到，请按 tools\manifest.json 取包。" -f $url) -ForegroundColor Yellow
            $pending++
            $rows += [pscustomobject]@{ 包 = $name; 结果 = '缺包'; 位置 = $url }
            continue
        }
        Write-Host ("   [核对] size={0} sha256={1}… 与清单一致" -f $wantSize, $wantSha.Substring(0, 16)) -ForegroundColor DarkGray

        # 保护层 2：包内文件与工作区里的跟踪文件同名却内容不同 → 同样跳过。
        $conflicts = @()
        if ($hasGit -and -not $Check) {
            Add-Type -AssemblyName System.IO.Compression.FileSystem -ErrorAction SilentlyContinue
            $conflicts = @(Get-ZipTrackedConflicts $zip $targetNorm $trackedSet)
        }
        if ($conflicts.Count -gt 0 -and -not $Force) {
            Write-Host ("   [跳过] 包内有 {0} 个文件与工作区里的跟踪文件不一致：解包会把它们降级回包内快照。" -f $conflicts.Count) -ForegroundColor Yellow
            foreach ($c in @($conflicts | Select-Object -First 3)) { Write-Host ("          {0}" -f $c) -ForegroundColor Yellow }
            if ($conflicts.Count -gt 3) { Write-Host ("          …… 共 {0} 个" -f $conflicts.Count) -ForegroundColor Yellow }
            Write-Host  '          要按包内快照覆盖请显式加 -Force。' -ForegroundColor Yellow
            $skipped++
            $rows += [pscustomobject]@{ 包 = $name; 结果 = '跳过(跟踪文件冲突)'; 位置 = $targetRel }
            continue
        }

        if ($Check) {
            Write-Host ("   [待解] {0} → {1}" -f $url, $targetRel) -ForegroundColor Yellow
            $rows += [pscustomobject]@{ 包 = $name; 结果 = '待解'; 位置 = ('{0} → {1}' -f $url, $targetRel) }
            continue
        }

        if ($conflicts.Count -gt 0) {
            Write-Host ("   [警告] -Force：将按包内快照覆盖 {0} 个跟踪文件（含 {1}）" -f $conflicts.Count, ($conflicts | Select-Object -First 1)) -ForegroundColor Magenta
        }
        New-Item -ItemType Directory -Force -Path $targetPath | Out-Null
        Write-Host ("   [解包] {0} → {1}" -f $url, $targetRel)
        Expand-Archive -LiteralPath $zip -DestinationPath $targetPath -Force
        if (Test-Path -LiteralPath $checkPath) {
            Write-Host ("   [完成] {0}" -f $checkRel) -ForegroundColor Green
            $rows += [pscustomobject]@{ 包 = $name; 结果 = '已解包'; 位置 = $checkRel }
        }
        else {
            Write-Host ("   [失败] 解包后仍找不到 {0}：包内容与清单不符？" -f $checkRel) -ForegroundColor Red
            $failed++
            $rows += [pscustomobject]@{ 包 = $name; 结果 = '失败'; 位置 = $checkRel }
        }
    }

    Write-Host ''
    Write-Host '---- 结果 ----' -ForegroundColor Cyan
    $rows | Format-Table -AutoSize | Out-String | Write-Host

    $goExe = Join-Path $RepoRoot 'tools\go\bin\go.exe'
    $moduleDir = Join-Path $RepoRoot 'server\work\dfo-lan'
    Write-Host '---- 下一步 ----' -ForegroundColor Cyan
    if (Test-Path -LiteralPath $goExe) {
        Write-Host ' 编译服务端（离线也能过，模块缓存在 tools\gopath）：'
        Write-Host ("   cd `"{0}`"" -f $moduleDir)
        Write-Host ("   `$env:GOPATH='{0}\tools\gopath'; `$env:GOMODCACHE='{0}\tools\gopath\pkg\mod'; `$env:GOPROXY='off'" -f $RepoRoot)
        Write-Host ("   & '{0}' build ./..." -f $goExe)
        Write-Host ("   产启动器： & '{0}' build -trimpath -o bin\dfolauncher.exe .\cmd\dfolauncher" -f $goExe)
    }
    else {
        Write-Host ' Go 工具链仍未就绪：先补齐 go 包，或用发布版启动器（自带 CLI）。' -ForegroundColor Yellow
    }
    Write-Host ' 看当前存储路线： scripts\storage-route.cmd show'
    Write-Host ' 启动游戏：       scripts\启动游戏.cmd（剧情）/ scripts\启动游戏-奥德赛.cmd（强制奥德赛档）'
    if ($skipped -gt 0) {
        Write-Host (" 注意：本轮按 git 工作区保护规则跳过 {0} 个包（见上面的 [跳过] 行）——工作区内容保持原样。" -f $skipped) -ForegroundColor Yellow
    }

    if ($failed -gt 0) { exit 3 }
    if ($pending -gt 0) { exit 2 }
    exit 0
}
catch {
    Write-Host ("[失败] " + $_.Exception.Message) -ForegroundColor Red
    exit 1
}

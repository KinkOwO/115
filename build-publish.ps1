# build-publish.ps1 - DFO 115us 单机发布包一键打包（相对路径，可反复运行）
# 用法: 一键打包.cmd                       使用默认发布分支打包
#       一键打包.cmd -Branch main          指定分支（默认 release/minimal-publish）
#       一键打包.cmd -OutDir D:\out        指定输出目录（默认仓库上一级）
#       一键打包.cmd -VerifyZip <zip 路径>  只校验一个已存在的包（不打新包；抽查/复验用）
# 流程: git 分支导出 -> 加入 tools/{pg}(排除 pgAdmin 4) ->
#       **复制预编译产物**(服务端程序 / 会话编排 CLI / WFP 探针 / configs) ->
#       launcher.local.json 模板(相对路径) -> Python 标准 zip(正斜杠/UTF-8) -> 逐项校验
[CmdletBinding()]
param(
    [string]$Branch = '',
    [string]$OutDir = '',
    [string]$VerifyZip = ''
)
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.IO.Compression.FileSystem

$ROOT = Split-Path -Parent $MyInvocation.MyCommand.Path
if (-not $Branch) { $Branch = 'release/minimal-publish' }
if (-not $OutDir) { $OutDir = Split-Path -Parent $ROOT }

# ==== 预编译产物清单（2026-10-05 业主硬性要求：玩家环境绝不编译，包必须自带这些文件）====
#
# 为什么必须在这里复制：`server/work/dfo-lan/bin/`（见该目录的 .gitignore）与 `*.exe`
# （见仓库根 .gitignore）都不入库，`git archive` 导出的树**不带**它们 —— 玩家拿到手一点
# 「开始游戏」就会报"缺少服务端 Go 编排 CLI / 服务端程序"。启动器虽然能按发布仓库的
# tools/manifest.json 自动补齐（见 internal/prebuilt），但**发布包本该自带**，别让玩家
# 为了开玩先下 22 MB。
$PrebuiltFiles = @(
    'server\work\dfo-lan\bin\dfolauncher.exe',   # Go 会话编排 CLI（launch / stop / prepare-inner-pvf / init-storage…）
    'server\work\dfo-lan\bin\wireprobe-pvf.exe', # 服务端程序（PVF 直读默认档 configs\pvf-default.json 指向它）
    'server\work\dfo_probe_tools\probe.exe'      # 客户端宿主回退路径（WFP 回环隔离后拉起客户端）
)
# 备份与半截文件绝不许进包：*.previous-* 是发布 PVF 默认程序时留下的旧版备份（36 MB 一份），
# *.exe~ 是编辑器/收尾工具留下的半成品 —— 它们白占体积，还可能被误当成可用程序。
$PrebuiltForbidden = @('*.previous-*', '*~')
# 服务端运行必需的配置目录（含不入库的大文件，如 configs\equipment-full.data）。
$ConfigsRel = 'server\work\dfo-lan\configs'

# Assert-PublishZip 逐项校验产物：关键文件必须在、configs 必须齐全、备份/半截文件必须不在。
# 缺任何一个就 throw 并说清缺什么 —— 绝不放行一个"跑不起来"的包。
function Assert-PublishZip {
    param([Parameter(Mandatory = $true)][string]$ZipPath)

    if (-not (Test-Path $ZipPath -PathType Leaf)) { throw "发布包不存在: $ZipPath" }
    $z = [IO.Compression.ZipFile]::OpenRead($ZipPath)
    try {
        $entries = @($z.Entries)
        $names = New-Object 'System.Collections.Generic.HashSet[string]'
        foreach ($e in $entries) { [void]$names.Add($e.FullName) }

        # (1) 关键条目：启动入口 + 配置样板 + 预编译产物 + 关键配置
        $need = @(
            'scripts/启动游戏.cmd',
            'server/launcher.local.json',
            'server/work/dfo-lan/configs/pvf-default.json'
        )
        foreach ($rel in $PrebuiltFiles) { $need += ($rel -replace '\\', '/') }
        if (Test-Path (Join-Path $ROOT 'tools\pg')) { $need += 'tools/pg/pgsql/bin/initdb.exe' }

        $miss = @()
        foreach ($n in $need) {
            if (-not $names.Contains($n)) { $miss += $n }
        }

        # (2) configs 全量比对：源目录里（除机器本地配置）有一个算一个，都必须出现在包里。
        #     只验几个代表文件是不够的 —— 少一个 configs 就可能让某个功能在玩家机器上哑掉。
        $cfgSrc = Join-Path $ROOT $ConfigsRel
        if (Test-Path $cfgSrc) {
            Get-ChildItem $cfgSrc -Recurse -File | ForEach-Object {
                if ($_.Name -like '*.local.json') { return }   # 机器本地配置（含口令）不进包
                $rel = $_.FullName.Substring($cfgSrc.Length).TrimStart('\').Replace('\', '/')
                $arc = 'server/work/dfo-lan/configs/' + $rel
                if (-not $names.Contains($arc)) { $miss += $arc }
            }
        }

        # (3) 禁止条目：bin\ 下的备份与半截文件
        $bad = @()
        foreach ($e in $entries) {
            $n = $e.FullName
            if ($n -notlike 'server/work/dfo-lan/bin/*') { continue }
            foreach ($pat in $PrebuiltForbidden) {
                if ($n -like $pat) { $bad += $n; break }
            }
        }

        if ($miss.Count -gt 0) {
            throw ("发布包缺少关键文件: " + ($miss -join ', ') +
                "`n  这些正是玩家'一点开始游戏就报错'的根因。先在工作区补齐（预编译产物见发布仓库的" +
                " server-bin 预编译包），再重新打包。")
        }
        if ($bad.Count -gt 0) {
            throw ("发布包里混进了备份/半截文件: " + ($bad -join ', ') +
                "`n  （*.previous-* 是旧程序备份、*.exe~ 是半成品）它们必须排除。")
        }

        # (4) 证据：把关键条目连大小一起列出来（业主看的就是这一段）
        Write-Host ("   校验通过: 共 {0} 个条目" -f $entries.Count)
        foreach ($n in $need) {
            $hit = $z.Entries | Where-Object { $_.FullName -eq $n } | Select-Object -First 1
            Write-Host ("     OK  {0}  ({1} bytes)" -f $n, $hit.Length)
        }
        $cfgShipped = @($entries | Where-Object { $_.FullName -like 'server/work/dfo-lan/configs/*' })
        Write-Host ("     OK  server/work/dfo-lan/configs/**  （{0} 个条目）" -f $cfgShipped.Count)
    } finally { $z.Dispose() }
}

# -VerifyZip：只校验一个已存在的包（不打新包）。业主抽查、或事后复验某个已下发的包时用。
if ($VerifyZip) {
    Write-Host "== 校验发布包: $VerifyZip =="
    Assert-PublishZip -ZipPath $VerifyZip
    Write-Host '[成功] 校验通过'
    return
}

git -C $ROOT rev-parse --verify --quiet $Branch 2>$null | Out-Null
if ($LASTEXITCODE -ne 0) { throw "分支不存在: $Branch（本地没有这个分支；用 -Branch main 或先建发布分支）" }

$stamp = Get-Date -Format 'yyyyMMdd'
$zipOut = Join-Path $OutDir "DFO-115US-单机发布包-$stamp.zip"
$work = Join-Path $env:TEMP "df-publish-$stamp"
$tar  = Join-Path $env:TEMP "df-publish-$stamp.tar.zip"

Write-Host "== 发布包打包: 分支 [$Branch] =="
Write-Host ("   输出: {0}" -f $zipOut)

if (Test-Path $work) { Remove-Item $work -Recurse -Force }
New-Item -ItemType Directory -Path $work | Out-Null
try {
    # 1. 从 git 分支导出发布内容（不依赖工作区状态）
    Write-Host '[1/6] git 分支导出...'
    git -C $ROOT archive --format=zip -o $tar $Branch
    if ($LASTEXITCODE -ne 0) { throw 'git archive 失败' }
    [IO.Compression.ZipFile]::ExtractToDirectory($tar, $work)

    # 2. 加入便携运行环境（pg，按需），排除 pgAdmin 4
    Write-Host '[2/6] 复制便携运行环境...'
    $tools = Join-Path $ROOT 'tools'
    New-Item -ItemType Directory -Path (Join-Path $work 'tools') | Out-Null
    foreach ($d in @('pg')) {
        $src = Join-Path $tools $d
        if (-not (Test-Path $src)) { Write-Warning "缺少运行环境目录 tools\$d (跳过)"; continue }
        Copy-Item $src (Join-Path $work "tools\$d") -Recurse -Force
    }
    $pga = Join-Path $work 'tools\pg\pgsql\pgAdmin 4'
    if (Test-Path $pga) { Remove-Item $pga -Recurse -Force; Write-Host '   已排除 pgAdmin 4' }

    # 3. 复制预编译产物 + configs（git 里没有这些文件，必须从工作区带进包）
    Write-Host '[3/6] 复制预编译产物（服务端程序 / 编排 CLI / 探针 / configs）...'
    $lackPrebuilt = @()
    foreach ($rel in $PrebuiltFiles) {
        $src = Join-Path $ROOT $rel
        if (-not (Test-Path $src -PathType Leaf)) { $lackPrebuilt += $rel; continue }
        $dst = Join-Path $work $rel
        New-Item -ItemType Directory -Path (Split-Path -Parent $dst) -Force | Out-Null
        Copy-Item $src $dst -Force
        $mb = [math]::Round((Get-Item $src).Length / 1MB, 1)
        Write-Host ("   + {0}  ({1} MB)" -f $rel, $mb)
    }
    if ($lackPrebuilt.Count -gt 0) {
        throw ("工作区里缺少预编译产物: " + ($lackPrebuilt -join ', ') +
            "`n  这些文件在 .gitignore 里（仓库不带），必须先在工作区造出来/取回来再打包：" +
            "`n   · bin\dfolauncher.exe 与 bin\wireprobe-pvf.exe：发布仓库的 server-bin 预编译包" +
            "`n   · dfo_probe_tools\probe.exe：该目录的 Build-Probe.cmd")
    }

    $cfgSrc = Join-Path $ROOT $ConfigsRel
    if (-not (Test-Path $cfgSrc -PathType Container)) {
        throw "工作区里缺少 $ConfigsRel（服务端运行必需的配置目录），无法打包"
    }
    $cfgDst = Join-Path $work $ConfigsRel
    New-Item -ItemType Directory -Path $cfgDst -Force | Out-Null
    $cfgCount = 0
    $cfgSkipped = @()
    Get-ChildItem $cfgSrc -Recurse -File | ForEach-Object {
        # 机器本地配置（server.local.json / launcher.local.json）含本机口令与绝对路径，绝不进包。
        if ($_.Name -like '*.local.json') { $cfgSkipped += $_.Name; return }
        $rel = $_.FullName.Substring($cfgSrc.Length).TrimStart('\')
        $dst = Join-Path $cfgDst $rel
        New-Item -ItemType Directory -Path (Split-Path -Parent $dst) -Force | Out-Null
        Copy-Item $_.FullName $dst -Force
        $cfgCount++
    }
    Write-Host ("   + {0}\**  ({1} 个文件)" -f $ConfigsRel, $cfgCount)
    if ($cfgSkipped.Count -gt 0) {
        Write-Host ("   - 已跳过机器本地配置: {0}" -f ($cfgSkipped -join ', '))
    }

    # 4. launcher.local.json 模板（相对路径）
    Write-Host '[4/6] 生成 launcher.local.json 模板...'
    Copy-Item (Join-Path $ROOT 'server\launcher.example.json') (Join-Path $work 'server\launcher.local.json') -Force

    # 5. Python 标准 zip 打包（正斜杠分隔符、UTF-8 文件名、无 ./ 前缀）
    Write-Host '[5/6] 压缩为 zip...'
    # Python 是 GM 工具专属依赖，已移出 tools\（启动链不需要它）：优先整合包外的 gm-tool\python，其次 PATH 上的 python。
    $py = Join-Path (Split-Path -Parent $ROOT) 'gm-tool\python\python.exe'
    if (-not (Test-Path $py)) {
        $cmdPy = Get-Command python -ErrorAction SilentlyContinue
        if ($cmdPy) { $py = $cmdPy.Source }
        else { throw '找不到 Python 解释器：gm-tool\python\python.exe 不存在，PATH 上也没有 python（Python 是 GM 工具专属依赖，已移出 tools\）' }
    }
    $pyScript = Join-Path $ROOT 'scripts\build_publish_zip.py'
    & $py $pyScript $work $zipOut
    if ($LASTEXITCODE -ne 0) { throw 'zip 打包失败' }

    # 6. 校验产物（缺任何一个就删掉半成品并说清缺什么）
    Write-Host '[6/6] 校验发布包...'
    try {
        Assert-PublishZip -ZipPath $zipOut
    } catch {
        Remove-Item $zipOut -Force -ErrorAction SilentlyContinue
        throw
    }

    $mb = [math]::Round((Get-Item $zipOut).Length / 1MB, 1)
    Write-Host ''
    Write-Host ("[成功] 发布包已生成: {0}  ({1} MB)" -f $zipOut, $mb)
    Write-Host '下一步: 解压后运行 scripts\启动游戏.cmd（首次会自动补齐服务端程序，并引导选择 DFO.exe）。'
} finally {
    Remove-Item $work -Recurse -Force -ErrorAction SilentlyContinue
    Remove-Item $tar -Force -ErrorAction SilentlyContinue
}

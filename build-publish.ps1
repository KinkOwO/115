# build-publish.ps1 - DFO 115us 最小发布包一键打包（相对路径，无绝对路径依赖）
# 用法: 一键打包.cmd                       使用当前 git 分支打包
#       一键打包.cmd -Branch release/minimal-publish   指定分支
#       一键打包.cmd -OutDir D:\out        指定输出目录（默认仓库上一级）
# 流程: git 分支导出 -> 加入 tools/{python,pg}(排除 pgAdmin 4) ->
#       launcher.local.json 模板(相对路径) -> Python 标准 zip(正斜杠/UTF-8) -> 关键文件校验
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
if ($LASTEXITCODE -ne 0) { throw "分支不存在: $Branch" }

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
    Write-Host '[1/5] git 分支导出...'
    git -C $ROOT archive --format=zip -o $tar $Branch
    if ($LASTEXITCODE -ne 0) { throw 'git archive 失败' }
    [IO.Compression.ZipFile]::ExtractToDirectory($tar, $work)

    # 2. 加入便携运行环境（python/pg），排除 pgAdmin 4
    Write-Host '[2/5] 复制便携运行环境...'
    $tools = Join-Path $ROOT 'tools'
    New-Item -ItemType Directory -Path (Join-Path $work 'tools') | Out-Null
    foreach ($d in @('python','pg')) {
        $src = Join-Path $tools $d
        if (-not (Test-Path $src)) { Write-Warning "缺少运行环境目录 tools\$d (跳过)"; continue }
        Copy-Item $src (Join-Path $work "tools\$d") -Recurse -Force
    }
    $pga = Join-Path $work 'tools\pg\pgsql\pgAdmin 4'
    if (Test-Path $pga) { Remove-Item $pga -Recurse -Force; Write-Host '   已排除 pgAdmin 4' }

    # 3. launcher.local.json 模板（相对路径）
    Write-Host '[3/5] 生成 launcher.local.json 模板...'
    Copy-Item (Join-Path $ROOT 'server\launcher.example.json') (Join-Path $work 'server\launcher.local.json') -Force

    # 4. Python 标准 zip 打包（正斜杠分隔符、UTF-8 文件名、无 ./ 前缀）
    Write-Host '[4/5] 压缩为 zip...'
    $py = Join-Path $ROOT 'tools\python\python.exe'
    if (-not (Test-Path $py)) { throw '缺少 tools\python\python.exe' }
    $pyScript = Join-Path $ROOT 'build_publish_zip.py'
    & $py $pyScript $work $zipOut
    if ($LASTEXITCODE -ne 0) { throw 'zip 打包失败' }

    # 5. 关键文件校验
    Write-Host '[5/5] 校验关键文件...'
    $z = [IO.Compression.ZipFile]::OpenRead($zipOut)
    try {
        $need = @('启动游戏.cmd','server/launcher.local.json','server/work/dfo-lan/bin/wireprobe-dungeon39.exe','server/work/dfo-lan/bin/wireprobe-handoff-source.exe','server/work/dfo-lan/configs/items.index.json','tools/python/python.exe','tools/pg/pgsql/bin/initdb.exe')
        $miss = @()
        foreach ($n in $need) {
            $hit = $z.Entries | Where-Object { $_.FullName -eq $n }
            if (-not $hit) { $miss += $n }
        }
        if ($miss.Count -gt 0) { Remove-Item $zipOut -Force -ErrorAction SilentlyContinue; throw "发布包缺少关键文件: $($miss -join ', ')" }
        Write-Host ("   校验通过: {0} 条目" -f $z.Entries.Count)
    } finally { $z.Dispose() }

    $mb = [math]::Round((Get-Item $zipOut).Length / 1MB, 1)
    Write-Host ''
    Write-Host ("[成功] 发布包已生成: {0}  ({1} MB)" -f $zipOut, $mb)
    Write-Host '下一步: 解压后先运行 配置环境.cmd 初始化, 再 启动服务端.cmd / 启动游戏.cmd。'
} finally {
    Remove-Item $work -Recurse -Force -ErrorAction SilentlyContinue
    Remove-Item $tar -Force -ErrorAction SilentlyContinue
}

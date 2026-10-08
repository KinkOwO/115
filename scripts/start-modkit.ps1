<#
.SYNOPSIS
  一键启动 modkit 简易页面（浏览器里管 mods 目录：分页列表 / 批量启停删 / 导入导出 zip）。

.DESCRIPTION
  默认管「仓库根的 mods\」（mod 作者工作区，认得 client-mods\*.zip 与 examples\<mod>\），
  也可以 -ServerMods 改管「服务端真正加载的 mods\」（server\work\dfo-lan\mods）。

  工具本体在 mods\modkit-web\：
    * 已经有 modkit-web.exe 就直接跑；
    * 没有（或 -Rebuild）就用 Go 编一份 —— Go 不在 PATH 时会去整合包的
      ..\tools\go\bin\go.exe 找（本机 go 不在 PATH 上是常态）。
    * 端口若已被占用，不重复起服务，只把地址打出来。
  * **默认不自动打开浏览器**（要开加 -Open）。

.PARAMETER ModsDir
  要管理的 mods 目录。给了它就按它来（默认认一层）。
.PARAMETER ServerMods
  改管服务端已装 mod 的目录（server\work\dfo-lan\mods），认一层。
.PARAMETER Port
  监听端口，默认 8931（只绑 127.0.0.1）。
.PARAMETER Addr
  监听地址，默认 127.0.0.1（不要随便改成 0.0.0.0：服务没有鉴权）。
.PARAMETER ScanDepth
  列表认几层：1 = 直接子目录；2 = 再往下一层。默认：作者工作区 2、其它 1。
.PARAMETER Open
  启动后顺手用默认浏览器打开页面。**默认不打开**（业主 2026-10-06：每次弹浏览器窗口很烦）。
.PARAMETER NoOpen
  兼容旧写法（等价于默认行为：不开浏览器）。
.PARAMETER Rebuild
  强制重新编译 modkit-web.exe。
.PARAMETER Help
  打印本帮助后退出。

.EXAMPLE
  scripts\启动modkit.cmd
.EXAMPLE
  scripts\启动modkit.cmd -ServerMods
.EXAMPLE
  scripts\启动modkit.cmd -Port 9000 -Open   # 想自动开页面时才加

.NOTES
  退出码：0 = 正常退出（含用户 Ctrl+C）；1 = 环境缺失（找不到工具目录 / 编译失败）。
  关掉这个窗口、或按 Ctrl+C 即停止服务。
#>
[CmdletBinding()]
param(
    [string]$ModsDir,
    [switch]$ServerMods,
    [int]$Port = 8931,
    [string]$Addr = '127.0.0.1',
    [int]$ScanDepth = 0,
    [switch]$Open,
    [switch]$NoOpen,  # 兼容旧命令：现在默认就不开浏览器，保留只为不报错
    [switch]$Rebuild,
    [switch]$Help
)

$ErrorActionPreference = 'Stop'

function Write-Head([string]$text) { Write-Host ''; Write-Host "== $text" -ForegroundColor Cyan }
function Write-Info([string]$text) { Write-Host "   $text" }
function Write-Bad([string]$text)  { Write-Host "   $text" -ForegroundColor Red }

if ($Help) {
    Get-Help $PSCommandPath -Detailed
    exit 0
}

$root    = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$toolDir = Join-Path $root 'mods\modkit-web'
$exe     = Join-Path $toolDir 'modkit-web.exe'
$srcMain = Join-Path $toolDir 'main.go'

Write-Head 'modkit 简易页面'
Write-Info "仓库根    ：$root"
Write-Info "工具目录  ：$toolDir"

if (-not (Test-Path $toolDir) -or -not (Test-Path $srcMain)) {
    Write-Head '启动失败'
    Write-Bad "找不到工具源码：$srcMain"
    Write-Bad '请确认 mods\modkit-web\ 存在（main.go / store.go / ui.html）。'
    exit 1
}

# ---- 目标目录与扫描层数 ----
$authorModsDir = Join-Path $root 'mods'
$serverModsDir = Join-Path $root 'server\work\dfo-lan\mods'
if (-not $ModsDir) {
    if ($ServerMods) { $ModsDir = $serverModsDir } else { $ModsDir = $authorModsDir }
}
if ($ScanDepth -le 0) {
    $ScanDepth = if ($ModsDir -eq $authorModsDir) { 2 } else { 1 }
}
if (-not (Test-Path $ModsDir)) {
    Write-Info "目录不存在，将按需创建：$ModsDir"
}
Write-Info "管理目录  ：$ModsDir（认 $ScanDepth 层）"

# ---- 找 Go（编译用；本机 go 常常不在 PATH 上）----
function Find-Go {
    $cmd = Get-Command go.exe -ErrorAction SilentlyContinue
    if ($cmd) { return $cmd.Source }
    foreach ($p in @(
            (Join-Path $root '..\tools\go\bin\go.exe'),
            'C:\Game\dof\115us\tools\go\bin\go.exe')) {
        if (Test-Path $p) { return (Resolve-Path $p).Path }
    }
    return $null
}

if ($Rebuild -or -not (Test-Path $exe)) {
    $go = Find-Go
    if (-not $go) {
        Write-Head '启动失败'
        Write-Bad '找不到 Go，且没有已编译好的 modkit-web.exe。'
        Write-Bad '请先装/指定 Go，或用打包内的 Go 编一份：'
        Write-Bad '  C:\Game\dof\115us\tools\go\bin\go.exe build -o mods\modkit-web\modkit-web.exe .'
        exit 1
    }
    Write-Head '编译 modkit-web'
    Write-Info "使用 Go：$go"
    Push-Location $toolDir
    try {
        & $go build -o $exe . 2>&1 | ForEach-Object { Write-Info $_ }
        if ($LASTEXITCODE -ne 0 -or -not (Test-Path $exe)) {
            Write-Bad "编译失败（exit=$LASTEXITCODE）"
            exit 1
        }
    } finally { Pop-Location }
    Write-Info "已生成：$exe"
}

# ---- 端口已被占用就不再起第二个 ----
$url = "http://${Addr}:$Port/"
$busy = $false
try {
    $busy = [bool](Get-NetTCPConnection -State Listen -LocalAddress $Addr -LocalPort $Port -ErrorAction SilentlyContinue)
} catch { $busy = $false }
if ($busy) {
    Write-Head '已在运行'
    Write-Info "端口 $Port 上已经有服务，不再起第二个实例。"
    Write-Info $url
    if ($Open) { Start-Process $url | Out-Null }
    exit 0
}

# ---- 启动（前台运行，Ctrl+C 即停）----
Write-Head '启动服务'
Write-Info $url
Write-Info '在浏览器里打开上面的地址操作；关掉本窗口或按 Ctrl+C 停止服务。'
Write-Host ''

$argv = @('--mods-dir', $ModsDir, '--scan-depth', "$ScanDepth", '--addr', "${Addr}:$Port")
# 默认**不**自动开浏览器（业主要求）；要开就加 -Open。
if ($Open) { $argv += '--open' }
& $exe @argv
exit $LASTEXITCODE

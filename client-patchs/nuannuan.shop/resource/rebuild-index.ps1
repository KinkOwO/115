param(
  [Parameter(Mandatory = $true)][string]$ClientDir,
  [string]$ToolsDir,
  [string]$PackDir,
  [string]$LogPath
)

# 重建 ImagePacks2\NpkIndex.etc —— NPK 集合一变，索引里的偏移就全部作废，
# 不重建 = 客户端一启动就闪退。
#
# 为什么不在本脚本里直接反射（踩过的坑，别再改回去）：
#   索引构建器 USLocalServer.Pvf.dll / USLocalServer.Launcher.Core.dll 的目标框架是
#   **net10.0**（依赖 System.Runtime 10.0.0.0）。而包内自带的 pwsh 7.4.6 便携版是
#   **自包含（self-contained）net8** 的（目录里有 hostfxr.dll / System.Private.CoreLib.dll），
#   自包含应用按设计忽略 roll-forward —— DOTNET_ROLL_FORWARD 环境变量、
#   runtimeconfig 里的 rollForward 都不生效，它永远跑在 8.0.10，加载这两个 DLL
#   必然报 Could not load file or assembly 'System.Runtime, Version=10.0.0.0'。
#   → 解决办法是包里带一个**自包含 net10 的小工具** tools/npkidx/NpkIndexTool.exe，
#     本脚本只负责找到它、复制到 %TEMP%、调用、原样透传输出与退出码。
#     目标机不需要装任何 .NET 运行时。
#
# 另两处必须知道的事：
#   1) 引擎执行索引脚本时只把 .ps1 抽到 <workDir>\.modkit-script\ 再跑，DLL 和 exe
#      都不会被拷过去 —— 所以工具目录 / 包根要由 mod.json 用 {pack} 显式传进来；
#   2) 115\ 树下加载可执行文件会被系统拦（hostfxr.dll 报 0x80070005），
#      所以 exe 与 DLL 一律先复制到 %TEMP% 再用。
#
# 退出码：0 通过；1 失败（引擎据此判定 npk.index 这一步失败）。

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'   # 别让 Copy-Item 的进度条混进引擎日志
try { [Console]::OutputEncoding = [Text.Encoding]::UTF8 } catch { }

$EXE_NAME = 'NpkIndexTool.exe'
$lines = New-Object System.Collections.Generic.List[string]

function Say([string]$msg) {
  $script:lines.Add($msg)
  Write-Host $msg
}

function Flush-Log {
  if ($LogPath) {
    try { $lines | Set-Content -LiteralPath $LogPath -Encoding UTF8 } catch { }
  }
}

# 候选路径里挑第一个真实存在的
function Pick-Existing([string[]]$cands) {
  foreach ($c in $cands) {
    if ($c -and (Test-Path -LiteralPath $c)) { return $c }
  }
  return $null
}

$ok = $false
try {
  $ClientDir = [IO.Path]::GetFullPath($ClientDir)
  $ip = Join-Path $ClientDir 'ImagePacks2'
  if (-not (Test-Path -LiteralPath $ip)) { throw ("客户端里没有 ImagePacks2 目录: " + $ip) }

  # ---- 反推 mod id（工作目录形如 ...\work\<mod-id>\indexN\.modkit-script）----
  $modId = ''
  try {
    $work = Split-Path -Parent $PSScriptRoot
    $modId = Split-Path -Leaf (Split-Path -Parent $work)
  }
  catch { }

  # ---- 定位 PVF\tools（索引构建器）----
  $toolCands = @()
  if ($ToolsDir) { $toolCands += $ToolsDir }
  if ($PackDir) { $toolCands += (Join-Path $PackDir 'resource\PVF\tools') }
  $toolCands += (Join-Path $PSScriptRoot 'PVF\tools')
  if ($modId) {
    $up = Split-Path -Parent $ClientDir
    foreach ($d in @(Get-ChildItem -LiteralPath $up -Directory -ErrorAction SilentlyContinue)) {
      $toolCands += (Join-Path $d.FullName ('mods\' + $modId + '\resource\PVF\tools'))
    }
  }
  $probe = Pick-Existing ($toolCands | ForEach-Object { Join-Path $_ 'USLocalServer.Pvf.dll' })
  if (-not $probe) {
    throw ("找不到索引构建器（资源目录里没有 USLocalServer.Pvf.dll）。已试：" + ($toolCands -join ' | '))
  }
  $tools = Split-Path -Parent $probe
  Say ("构建器目录 : " + $tools)

  # ---- 定位自包含 net10 的索引工具 ----
  $exeCands = @()
  if ($PackDir) { $exeCands += (Join-Path $PackDir ('tools\npkidx\' + $EXE_NAME)) }
  $exeCands += (Join-Path $PSScriptRoot ('tools\npkidx\' + $EXE_NAME))
  if ($modId) {
    $up = Split-Path -Parent $ClientDir
    foreach ($d in @(Get-ChildItem -LiteralPath $up -Directory -ErrorAction SilentlyContinue)) {
      $exeCands += (Join-Path $d.FullName ('mods\' + $modId + '\tools\npkidx\' + $EXE_NAME))
    }
  }
  $exeCands += (Join-Path $env:TEMP ('dfomod-npkidx\' + $EXE_NAME))
  $exe = Pick-Existing $exeCands
  if (-not $exe) {
    throw ("找不到 " + $EXE_NAME + "（包内 tools\npkidx\）。已试：" + ($exeCands -join ' | '))
  }
  Say ("索引工具   : " + $exe)

  # ---- 复制到 %TEMP%（115\ 树下加载 exe / 反射加载 DLL 都会被拦）----
  $tmpExeDir = Join-Path $env:TEMP 'dfomod-npkidx'
  $tmpToolDir = Join-Path $env:TEMP 'dfomod-idxtools'
  New-Item -ItemType Directory -Force -Path $tmpExeDir | Out-Null
  New-Item -ItemType Directory -Force -Path $tmpToolDir | Out-Null
  if ((Split-Path -Parent $exe) -ne $tmpExeDir) {
    Copy-Item -LiteralPath $exe -Destination (Join-Path $tmpExeDir $EXE_NAME) -Force
  }
  Copy-Item -Path (Join-Path $tools '*.dll') -Destination $tmpToolDir -Force

  Say ("客户端目录 : " + $ClientDir)
  Say ("图像包目录 : " + $ip)
  Say ""

  # ---- 跑（输出直接透传到 stdout，引擎会一起收进日志）----
  & (Join-Path $tmpExeDir $EXE_NAME) '--ip' $ip '--tools' $tmpToolDir
  $rc = $LASTEXITCODE
  Say ""
  Say ("[script] 索引工具退出码 " + $rc)
  $ok = ($rc -eq 0)
}
catch {
  Say ("[FAIL] 出错中断：" + $_.Exception.Message)
  $ok = $false
}
finally {
  Flush-Log
}

if (-not $ok) { exit 1 }
exit 0

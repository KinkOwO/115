# 重新编译索引重建小工具（自包含 net10 单文件 exe）。
#
# 为什么要有它：mod 自带的索引构建器 USLocalServer.*.dll 目标框架 net10.0，
# 而包内那个 pwsh 便携版是自包含 net8（忽略 roll-forward，永远 8.0.10），
# 加载不了它们 —— 只能用一个跑在 net10 上的独立 exe 来调用。
#
# 用法：
#   .\publish.ps1                       # 自动找 %TEMP%\dotnet-sdk10 或 PATH 里的 dotnet
#   .\publish.ps1 -Dotnet <dotnet.exe>  # 指定 SDK
#
# 产物写到 <整合包根>\.tmp\npkidx\NpkIndexTool.exe（build-mod.py 从那里取）。
# 退出码：0 成功；1 失败。

param([string]$Dotnet)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

$here = Split-Path -Parent $MyInvocation.MyCommand.Path
$repo = Resolve-Path (Join-Path $here '..\..')
$out = Join-Path $repo '.tmp\npkidx'
$proj = Join-Path $here 'NpkIndexTool.csproj'

if (-not $Dotnet) {
  foreach ($c in @("$env:TEMP\dotnet-sdk10\dotnet.exe", "$env:LOCALAPPDATA\Microsoft\dotnet\dotnet.exe")) {
    if (Test-Path -LiteralPath $c) { $Dotnet = $c; break }
  }
}
if (-not $Dotnet) {
  $cmd = Get-Command dotnet -ErrorAction SilentlyContinue
  if ($cmd) { $Dotnet = $cmd.Source }
}
if (-not $Dotnet) {
  Write-Host '[FAIL] 找不到 .NET 10 SDK。先装：'
  Write-Host '       iex "& { $(irm https://dot.net/v1/dotnet-install.ps1) } -Channel 10.0 -InstallDir $env:TEMP\dotnet-sdk10 -NoPath"'
  exit 1
}

$ver = & $Dotnet --version
Write-Host ("dotnet  : " + $Dotnet + "  (" + $ver + ")")
Write-Host ("产物    : " + $out)

& $Dotnet publish $proj -c Release -o $out
if ($LASTEXITCODE -ne 0) { Write-Host '[FAIL] publish 失败'; exit 1 }

$exe = Join-Path $out 'NpkIndexTool.exe'
if (-not (Test-Path -LiteralPath $exe)) { Write-Host '[FAIL] 没产出 exe'; exit 1 }
Write-Host ("完成    : " + $exe + "  " + [int]((Get-Item $exe).Length / 1MB) + " MB")
exit 0

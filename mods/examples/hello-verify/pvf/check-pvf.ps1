param(
  [Parameter(Mandatory = $true)][string]$ClientDir,
  [Parameter(Mandatory = $true)][string]$OutputDir
)

# hello-verify 的 pvf 层：**只读干跑校验**。
#
# 它证明 pvf 层的 verify 通路可用：在不动任何文件的条件下检查客户端的
# Script.pvf / sk.dat 是否可读、是否成对、大小与 sha256 是多少，
# 并把结果写成报告供人在 OutputDir 里核对。
#
# 退出码：0 = 通过；非 0 = 失败（引擎会据此拒绝安装）。

$ErrorActionPreference = 'Stop'

function Fail([string]$msg) {
  Write-Host "pvf check FAILED: $msg"
  exit 1
}

if (-not (Test-Path -LiteralPath $ClientDir)) {
  Fail "client dir not found: $ClientDir"
}
if (-not (Test-Path -LiteralPath $OutputDir)) {
  New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null
}

$pvf = Join-Path $ClientDir 'Script.pvf'
$sk  = Join-Path $ClientDir 'sk.dat'

foreach ($pair in @(
    @{ Path = $pvf; Label = 'Script.pvf' },
    @{ Path = $sk;  Label = 'sk.dat' })) {
  if (-not (Test-Path -LiteralPath $pair.Path)) {
    Fail "$($pair.Label) missing at $($pair.Path)"
  }
  $item = Get-Item -LiteralPath $pair.Path
  if ($item.Length -le 0) {
    Fail "$($pair.Label) is empty"
  }
}

# Script.pvf 与 sk.dat 必须成对：只换一个会让客户端与内层归档不匹配，
# 所以校验脚本也把"两个都在"当作通过条件。
$rows = foreach ($f in @($pvf, $sk)) {
  $item = Get-Item -LiteralPath $f
  $hash = (Get-FileHash -LiteralPath $f -Algorithm SHA256).Hash.ToLower()
  [pscustomobject]@{
    file   = $item.Name
    bytes  = $item.Length
    sha256 = $hash
  }
}

# 顺带确认这不是空壳归档：真实 Script.pvf 远大于 1 KiB。
$pvfSize = (Get-Item -LiteralPath $pvf).Length
if ($pvfSize -lt 1024) {
  Fail "Script.pvf looks too small ($pvfSize bytes) - not a real archive"
}

$report = Join-Path $OutputDir 'pvf-check-report.txt'
$lines = @()
$lines += "hello-verify pvf layer check (read-only)"
$lines += "client   : $ClientDir"
$lines += "checked  : $(Get-Date -Format s)"
$lines += ""
$rows | ForEach-Object {
  $lines += ("{0,-14} {1,12} bytes  sha256={2}" -f $_.file, $_.bytes, $_.sha256)
}
$lines += ""
$lines += "result   : PASS (both files present, paired, non-empty)"
$lines | Set-Content -LiteralPath $report -Encoding UTF8

Write-Host "pvf check PASSED"
$rows | ForEach-Object { Write-Host ("  {0} {1} bytes {2}" -f $_.file, $_.bytes, $_.sha256) }
Write-Host "report: $report"
exit 0

# DFO 115us GM 管理台启动脚本（gm-tool 自足版）
# 用途：确保 gmweb(28080) 运行 -> 启动 Dashboard 代理(28081) -> 打开浏览器
# 位置：D:\115us\gm-tool\dashboard\start_gm_dashboard.ps1
$ErrorActionPreference = 'Continue'
$gt    = Split-Path -Parent $PSScriptRoot          # gm-tool 目录
$root  = Split-Path -Parent $gt                    # D:\115us
$py    = Join-Path $root 'tools\python\python.exe'
if (-not (Test-Path -LiteralPath $py)) { $py = Join-Path $gt 'python\python.exe' }
$proxy    = Join-Path $PSScriptRoot 'gm_dashboard_proxy.py'
$gmwebCmd = Join-Path $gt 'Start-GMWeb.cmd'
$dashUrl  = 'http://127.0.0.1:28081/#accounts'

function Port-Listen([int]$port) {
  return [bool](Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue)
}

# 1. gmweb 是否在跑
if (-not (Port-Listen 28080)) {
  Write-Host 'GM 工具（gmweb 28080）未运行，正在启动...'
  if (Test-Path -LiteralPath $gmwebCmd) {
    Start-Process -FilePath 'cmd.exe' -ArgumentList '/c', "`"$gmwebCmd`" --no-browser" -WindowStyle Normal
  } else {
    Write-Host '未找到 Start-GMWeb.cmd，请手动启动 GM 工具后重试。'
    Start-Sleep -Seconds 5
    exit 1
  }
  $ok = $false
  for ($i = 0; $i -lt 40; $i++) {
    Start-Sleep -Milliseconds 500
    if (Port-Listen 28080) { $ok = $true; break }
  }
  if (-not $ok) {
    Write-Host 'GM 工具启动超时（28080 未监听）。请检查 gm-tool 是否正常。'
    Start-Sleep -Seconds 5
    exit 1
  }
} else {
  Write-Host 'GM 工具已在运行（28080）。'
}

# 2. 管理台代理是否已运行
if (Port-Listen 28081) {
  Write-Host 'GM 管理台已在运行：' $dashUrl
} else {
  Write-Host '启动管理台代理（28081）...'
  Start-Process -FilePath $py -ArgumentList $proxy -WindowStyle Hidden
  $ok = $false
  for ($i = 0; $i -lt 20; $i++) {
    Start-Sleep -Milliseconds 500
    if (Port-Listen 28081) { $ok = $true; break }
  }
  if (-not $ok) {
    Write-Host '管理台代理启动失败（28081 未监听）。'
    Start-Sleep -Seconds 5
    exit 1
  }
}

# 3. 打开浏览器
Start-Process $dashUrl
Write-Host '已打开 GM 管理台：' $dashUrl

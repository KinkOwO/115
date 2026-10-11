<#
.SYNOPSIS
  在临时夹具中验证停止环境的租约收尾，不停止真实进程、不访问玩家库。
.DESCRIPTION
  只加载 storage-route.ps1 的相关函数，使用模拟停止器及进程查询。
  退出码：0 通过；1 回归失败。
#>
$ErrorActionPreference = 'Stop'
$RepoRoot = Split-Path -Parent $PSScriptRoot
Set-Location -LiteralPath $RepoRoot
$tokens = $null
$parseErrors = $null
$ast = [Management.Automation.Language.Parser]::ParseFile((Join-Path $PSScriptRoot 'storage-route.ps1'), [ref]$tokens, [ref]$parseErrors)
if ($parseErrors.Count) { throw ($parseErrors | Out-String) }
foreach ($name in @('Read-JsonFile', 'ConvertTo-OrderedTable', 'Get-StorageDriver', 'Clear-AdminGuard', 'Invoke-EnvironmentStop')) {
    $definition = $ast.Find({ param($node) $node -is [Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq $name }, $true)
    if (-not $definition) { throw "缺少函数：$name" }
    . ([scriptblock]::Create($definition.Extent.Text))
}
$fixture = Join-Path $RepoRoot ('.tmp\stop-environment-tests\' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $fixture | Out-Null
$ActiveFile = Join-Path $fixture 'local.json'
$dbPath = Join-Path $fixture 'fixture.sqlite3'
$leasePath = "$dbPath.admin-guard"
$utf8 = New-Object Text.UTF8Encoding($false)
[IO.File]::WriteAllText($ActiveFile, (@{ driver = 'sqlite'; sqlite_path = $dbPath } | ConvertTo-Json), $utf8)
[IO.File]::WriteAllText($dbPath, 'save sentinel', $utf8)
$script:queryMode = 'gone'
$script:stopCode = 0
$script:stops = 0
function Invoke-StopLauncher([string[]]$Arguments) {
    $script:stops++
    return $script:stopCode
}
function Write-EntryLog([string]$verb, [string]$message) { Write-Host $message }
function Get-Process {
    param([int]$Id)
    switch ($script:queryMode) {
        'gone' { Write-Error 'fixture holder is gone' -ErrorId 'NoProcessFoundForGivenId' -Category ObjectNotFound -ErrorAction Stop }
        'alive' { return [pscustomobject]@{ ProcessName = 'fixture-writer' } }
        'unknown' { throw 'fixture access denied' }
    }
}
function Assert([bool]$condition, [string]$message) { if (-not $condition) { throw $message } }
function Put-Lease([string]$holder = '999999') { [IO.File]::WriteAllText($leasePath, $holder, $utf8) }
function Expect-Failure([scriptblock]$action, [string]$pattern) {
    $failure = $null
    try { & $action } catch { $failure = $_.Exception.Message }
    Assert ($failure -and $failure -like $pattern) "应失败 '$pattern'，实际：$failure"
}
Put-Lease
Invoke-EnvironmentStop @('--dry-run')
Assert (Test-Path -LiteralPath $leasePath) 'dry-run 不得删除租约'
Invoke-EnvironmentStop @()
Assert (-not (Test-Path -LiteralPath $leasePath)) '已退出持有者的租约必须清理'
Invoke-EnvironmentStop @()
Put-Lease
Invoke-EnvironmentStop $null
Assert (-not (Test-Path -LiteralPath $leasePath)) '入口的 null 参数绑定必须作为无参数处理'
Put-Lease
Invoke-EnvironmentStop @('')
Assert (-not (Test-Path -LiteralPath $leasePath)) '提权转交的空字符串必须作为无参数处理'
foreach ($mode in @('alive', 'unknown')) {
    Put-Lease
    $script:queryMode = $mode
    Expect-Failure { Invoke-EnvironmentStop @() } '*租约*'
    Assert (Test-Path -LiteralPath $leasePath) "$mode 状态必须保留租约并失败"
}
$script:queryMode = 'gone'
Put-Lease 'invalid'
Expect-Failure { Invoke-EnvironmentStop @() } '*没有可用的 pid*'
Assert (Test-Path -LiteralPath $leasePath) '非法 pid 必须保留'
Put-Lease
$script:stopCode = 1
Expect-Failure { Invoke-EnvironmentStop @() } '*停止链未完成*'
Assert (-not (Test-Path -LiteralPath $leasePath)) '停止器失败后仍应检查残留租约'
Put-Lease
Expect-Failure { Invoke-EnvironmentStop @('--dry-run') } '*停止预检失败*'
Assert (Test-Path -LiteralPath $leasePath) '失败的预检也不得删除租约'
$before = $script:stops
Expect-Failure { Invoke-EnvironmentStop @('--check') } '*不支持参数*'
Assert ($script:stops -eq $before) '非法参数不得执行停止器'
Remove-Item Function:\Get-Process
$script:stopCode = 0
Put-Lease '2147483647'
Invoke-EnvironmentStop @()
Assert (-not (Test-Path -LiteralPath $leasePath)) '真实进程查询必须识别不存在的 pid'
Put-Lease ([string]$PID)
Expect-Failure { Invoke-EnvironmentStop @() } '*仍在运行*'
Assert (Test-Path -LiteralPath $leasePath) '真实存活进程的夹具租约必须保留'
Assert ([IO.File]::ReadAllText($dbPath) -eq 'save sentinel') '存档夹具必须保留原文'
Write-Host 'PASS: dry-run / stale / alive / unknown / invalid / stop failure / save preservation'

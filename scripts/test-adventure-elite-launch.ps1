<#
.SYNOPSIS
  验证精锐开关路由与提权参数传递，不执行 storage-route 主体，不启动服务端或游戏。
.NOTES
  仅写本脚本拥有的系统临时目录；退出码 0 通过，1 失败。
#>
[CmdletBinding()]
param()
$ErrorActionPreference = 'Stop'
$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
Set-Location -LiteralPath $RepoRoot
$routePath = Join-Path $PSScriptRoot 'storage-route.ps1'
$parseTokens = $null
$parseErrors = $null
$ast = [Management.Automation.Language.Parser]::ParseFile($routePath, [ref]$parseTokens, [ref]$parseErrors)
if ($parseErrors.Count -ne 0) { throw ($parseErrors | Out-String) }
foreach ($name in @('Get-GoLauncherPath', 'Get-EncodedLaunchHandoff')) {
    $definitions = @($ast.FindAll({ param($node) $node -is [Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq $name }, $true))
    if ($definitions.Count -ne 1) { throw ('无法找到唯一函数：' + $name) }
    # Execute function definitions only. No launch/storage/main-body statement runs.
    . ([ScriptBlock]::Create($definitions[0].Extent.Text))
}
$savedElite = [Environment]::GetEnvironmentVariable('DFO_ADVENTURE_ELITE', 'Process')
$savedOdyssey = [Environment]::GetEnvironmentVariable('DFO_ODYSSEY_MODE', 'Process')
$testBase = [IO.Path]::GetFullPath((Join-Path ([IO.Path]::GetTempPath()) 'dfo-elite-launch-tests'))
$testDirectory = [IO.Path]::GetFullPath((Join-Path $testBase ([Guid]::NewGuid().ToString('N'))))
if (!$testDirectory.StartsWith($testBase + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) { throw '临时目录越界' }
try {
    New-Item -ItemType Directory -Path $testDirectory -Force | Out-Null
    $fixture = Join-Path $testDirectory "测试 ' 入口.ps1"
    $body = @'
param([Parameter(ValueFromRemainingArguments=$true)][string[]]$Rest)
[pscustomobject]@{Elite=$env:DFO_ADVENTURE_ELITE;Odyssey=$env:DFO_ODYSSEY_MODE;Arguments=@($Rest);Root=(Get-Location).Path} | ConvertTo-Json -Compress
exit 0
'@
    [IO.File]::WriteAllText($fixture, $body, [Text.UTF8Encoding]::new($true))
    foreach ($value in @('', '0', '1', 'true', "1'; exit 90; #")) {
        [Environment]::SetEnvironmentVariable('DFO_ADVENTURE_ELITE', $value, 'Process')
        $selected = Get-GoLauncherPath
        $expected = if ($value -ceq '1') { 'dfolauncher-adventure-elite.exe' } else { 'dfolauncher.exe' }
        if ([IO.Path]::GetFileName($selected) -cne $expected) { throw ('路由错误：' + $value) }
        $env:DFO_ODYSSEY_MODE = '1'
        $arguments = @('--tag', "测试 ' & 记录")
        $encoded = Get-EncodedLaunchHandoff 'game-sqlite' $arguments $fixture
        # Simulate UAC losing the caller's process environment before running
        # the exact generated command in a normal PowerShell 5.1 child.
        [Environment]::SetEnvironmentVariable('DFO_ADVENTURE_ELITE', $null, 'Process')
        [Environment]::SetEnvironmentVariable('DFO_ODYSSEY_MODE', $null, 'Process')
        $output = & powershell.exe -NoProfile -ExecutionPolicy Bypass -EncodedCommand $encoded
        if ($LASTEXITCODE -ne 0) { throw ('传递失败：exit=' + $LASTEXITCODE) }
        $actual = ($output -join "`n") | ConvertFrom-Json
        if ([string]$actual.Elite -cne $value -or $actual.Odyssey -cne '1') { throw '环境值未完整传递' }
        if (($actual.Arguments -join '|') -cne (@('game-sqlite') + $arguments -join '|')) { throw 'Unicode 参数未完整传递' }
        if ($actual.Root -cne $RepoRoot) { throw '工作目录错误' }
    }
    Write-Host 'PASS: exact switch routing, lost-environment handoff, Unicode arguments, quoted values; no game/server launched.'
} finally {
    [Environment]::SetEnvironmentVariable('DFO_ADVENTURE_ELITE', $savedElite, 'Process')
    [Environment]::SetEnvironmentVariable('DFO_ODYSSEY_MODE', $savedOdyssey, 'Process')
    # Checked absolute target above; only this test's GUID directory is removed.
    if (Test-Path -LiteralPath $testDirectory) { Remove-Item -LiteralPath $testDirectory -Recurse }
}

<#
.SYNOPSIS
  构建精锐资格候选、执行本进程机制测试并打包；不安装、不启动客户端。
.NOTES
  -ClientExe 只读提供离线安装夹具的原生代码字节，测试不启动该程序。
  退出码：0 全部完成；1 环境或构建/测试/打包失败。默认写入补丁 dist/；-WithServer 另构建隔离启动器及服务端候选，不覆盖默认程序。
#>
[CmdletBinding()]
param(
    [string]$VCVars = 'D:\VS2026\VC\Auxiliary\Build\vcvars64.bat',
    [string]$Python = 'D:\ProgramFiles\Python310\python.exe',
    [switch]$WithServer,
    [string]$ClientExe,
    [string]$Go = (Join-Path $env:USERPROFILE 'go\pkg\mod\golang.org\toolchain@v0.0.1-go1.26.0.windows-amd64\bin\go.exe')
)
$ErrorActionPreference = 'Stop'
$taskRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
Set-Location -LiteralPath $taskRoot
$patchDir = Join-Path $taskRoot 'client-patchs\adventure-elite'
$distDir = Join-Path $patchDir 'dist'
if (!(Test-Path -LiteralPath $VCVars) -or !(Test-Path -LiteralPath $Python)) {
    throw '缺少 MSVC x64 环境或 Python；请用 -VCVars / -Python 指定。'
}
# Refuse shell metacharacters in caller-provided executable paths.
if ($VCVars -match '["&|<>^%\r\n]' -or $taskRoot -match '["&|<>^%\r\n]') {
    throw '构建路径含不支持的命令字符。'
}
New-Item -ItemType Directory -Path $distDir -Force | Out-Null
Push-Location -LiteralPath $distDir
try {
    $minhookDir = Join-Path $patchDir 'vendor\minhook'
    $vendorSources = @('src\buffer.c', 'src\hook.c', 'src\trampoline.c', 'src\hde\hde64.c') | ForEach-Object { '"' + (Join-Path $minhookDir $_) + '"' }
    $compileVendor = 'call "' + $VCVars + '" >nul && cl /nologo /c /TC /W3 /O2 /MT /utf-8 ' + ($vendorSources -join ' ')
    & cmd.exe /d /s /c $compileVendor
    if ($LASTEXITCODE -ne 0) { throw 'MinHook source build failed.' }
    $hookObjects = ' buffer.obj hook.obj trampoline.obj hde64.obj'
    $assemble = 'call "' + $VCVars + '" >nul && ml64 /nologo /c /Focondition-relay.obj "' + (Join-Path $patchDir 'src\condition-relay.asm') + '"'
    & cmd.exe /d /s /c $assemble
    if ($LASTEXITCODE -ne 0) { throw 'Condition relay assembly failed.' }
    $compileDll = 'call "' + $VCVars + '" >nul && cl /nologo /std:c++17 /W4 /WX /O2 /MT /LD /utf-8 "' + (Join-Path $patchDir 'src\adventure-elite.cpp') + '" /Fe:AdventureElite.dll /Fo:adventure-elite.obj' + $hookObjects + ' condition-relay.obj /link /INCREMENTAL:NO'
    & cmd.exe /d /s /c $compileDll
    if ($LASTEXITCODE -ne 0) { throw 'DLL 构建失败。' }
    $compileTest = 'call "' + $VCVars + '" >nul && cl /nologo /std:c++17 /W4 /WX /O2 /MT /utf-8 "' + (Join-Path $patchDir 'src\eligibility-test.cpp') + '" /Fe:eligibility-test.exe /Fo:eligibility-test.obj /link /INCREMENTAL:NO'
    & cmd.exe /d /s /c $compileTest
    if ($LASTEXITCODE -ne 0) { throw '机制测试构建失败。' }
    & (Join-Path $distDir 'eligibility-test.exe')
    if ($LASTEXITCODE -ne 0) { throw '机制测试失败。' }
    $compileObserveTest = 'call "' + $VCVars + '" >nul && cl /nologo /std:c++17 /W4 /WX /O2 /MT /utf-8 "' + (Join-Path $patchDir 'src\notification-observer-test.cpp') + '" /Fe:notification-observer-test.exe /Fo:notification-observer-test.obj /link /INCREMENTAL:NO'
    & cmd.exe /d /s /c $compileObserveTest
    if ($LASTEXITCODE -ne 0) { throw '通知观察机制测试构建失败。' }
    & (Join-Path $distDir 'notification-observer-test.exe')
    if ($LASTEXITCODE -ne 0) { throw '通知观察机制测试失败。' }
    foreach ($testName in @('preparation-state-test', 'preparation-hook-test', 'trace-record-test', 'lifecycle-snapshot-test', 'lifecycle-trace-test', 'native-completion-test', 'entry-trace-test', 'combat-trace-test', 'combat-ownership-test')) {
        $compilePreparation = 'call "' + $VCVars + '" >nul && cl /nologo /std:c++17 /W4 /WX /O2 /MT /utf-8 "' + (Join-Path $patchDir ('src\' + $testName + '.cpp')) + '" /Fe:' + $testName + '.exe /Fo:' + $testName + '.obj' + $hookObjects + ' /link /INCREMENTAL:NO'
        & cmd.exe /d /s /c $compilePreparation
        if ($LASTEXITCODE -ne 0) { throw ($testName + ' build failed.') }
        & (Join-Path $distDir ($testName + '.exe'))
        if ($LASTEXITCODE -ne 0) { throw ($testName + ' failed.') }
    }
    $fixtureAsm = 'call "' + $VCVars + '" >nul && ml64 /nologo /c /Focondition-fixture.obj "' + (Join-Path $patchDir 'src\condition-fixture.asm') + '"'
    & cmd.exe /d /s /c $fixtureAsm
    if ($LASTEXITCODE -ne 0) { throw 'Condition fixture assembly failed.' }
    $registrationTest = 'call "' + $VCVars + '" >nul && cl /nologo /std:c++17 /W4 /WX /O2 /MT /utf-8 "' + (Join-Path $patchDir 'src\registration-mechanism-test.cpp') + '" /Fe:registration-mechanism-test.exe /Fo:registration-mechanism-test.obj condition-relay.obj condition-fixture.obj /link /INCREMENTAL:NO'
    & cmd.exe /d /s /c $registrationTest
    if ($LASTEXITCODE -ne 0) { throw 'Registration mechanism build failed.' }
    & (Join-Path $distDir 'registration-mechanism-test.exe')
    if ($LASTEXITCODE -ne 0) { throw ('Registration mechanism failed: ' + $LASTEXITCODE) }
    $startupTest = 'call "' + $VCVars + '" >nul && cl /nologo /std:c++17 /W4 /WX /O2 /MT /utf-8 "' + (Join-Path $patchDir 'src\registration-startup-test.cpp') + '" /Fe:registration-startup-test.exe /Fo:registration-startup-test.obj' + $hookObjects + ' condition-relay.obj /link /INCREMENTAL:NO'
    & cmd.exe /d /s /c $startupTest
    if ($LASTEXITCODE -ne 0) { throw 'Registration startup fixture build failed.' }
    if (!$ClientExe) { $ClientExe = Join-Path $taskRoot 'client\DFO.exe' }
    if (!(Test-Path -LiteralPath $ClientExe -PathType Leaf)) { throw 'Missing client bytes for offline startup fixture; specify -ClientExe.' }
    & (Join-Path $distDir 'registration-startup-test.exe') $ClientExe
    if ($LASTEXITCODE -ne 0) { throw ('Registration startup fixture failed: ' + $LASTEXITCODE) }
    & $Python (Join-Path $patchDir 'build-mod.py')
    if ($LASTEXITCODE -ne 0) { throw 'mod 打包失败。' }
    Write-Host '资格 DLL 构建、机制测试和打包完成；包未安装。'
} finally { Pop-Location }


if ($WithServer) {
    if (!(Test-Path -LiteralPath $Go -PathType Leaf)) { throw '缺少 Go 1.26，请用 -Go 指定。' }
    $goVersion = & $Go version
    if ($LASTEXITCODE -ne 0 -or $goVersion -notmatch 'go1\.26\.') { throw ('必须使用 Go 1.26：' + $goVersion) }
    $env:GOTOOLCHAIN = 'local'
    $env:GOPROXY = 'https://goproxy.cn,direct'
    $env:GOPATH = 'C:\Game\dof\115us\tools\gopath'
    $env:GOMODCACHE = Join-Path $env:USERPROFILE 'go\pkg\mod'
    $env:GOCACHE = Join-Path $taskRoot '.tmp\adventure-elite\gocache'
    Push-Location (Join-Path $taskRoot 'server\work\dfo-lan')
    try {
        & $Go build -trimpath -o bin/wireprobe-handoff-source.exe ./cmd/wireprobe
        if ($LASTEXITCODE -ne 0) { throw '服务端候选构建失败。' }
        & $Go build -trimpath -o bin/dfolauncher-adventure-elite.exe ./cmd/dfolauncher
        if ($LASTEXITCODE -ne 0) { throw '精锐资格启动器候选构建失败。' }
    } finally { Pop-Location }
    Write-Host '隔离候选已构建。DFO_ADVENTURE_ELITE=1 后由玩家运行启动游戏脚本；未启动游戏。'
}

# Candidate identity accompanies the DLL. CheckOnly can validate without launching.
$artifactRows = @()
foreach ($artifactPath in @('client-patchs/adventure-elite/dist/AdventureElite.dll', 'server/work/dfo-lan/bin/wireprobe-handoff-source.exe', 'server/work/dfo-lan/bin/dfolauncher-adventure-elite.exe')) {
    $artifactFile = Join-Path $taskRoot $artifactPath
    $artifactHash = if (Test-Path -LiteralPath $artifactFile -PathType Leaf) { (Get-FileHash -LiteralPath $artifactFile -Algorithm SHA256).Hash.ToLowerInvariant() } else { '' }
    $artifactRows += [ordered]@{ path = $artifactPath; sha256 = $artifactHash }
}
$resourceRows = @()
foreach ($resourcePath in @('client/DFO.exe', 'client/Script.pvf', 'client/sk.dat', 'server/work/client-build/Script.inner.pvf')) {
    $resourceFile = Join-Path $taskRoot $resourcePath
    if (Test-Path -LiteralPath $resourceFile -PathType Leaf) {
        $resourceRows += [ordered]@{ path = $resourcePath; bytes = (Get-Item -LiteralPath $resourceFile).Length; sha256 = (Get-FileHash -LiteralPath $resourceFile -Algorithm SHA256).Hash.ToLowerInvariant() }
    }
}
$buildIdentity = [ordered]@{ version = '0.3.10'; changeFrom030 = 'ordinary-owned-combat-candidate'; serverCandidateStage = 'ordinary-reentry'; serverCandidateAttempt = '1/3'; registrationReentryAttempt = '2/3'; generatedAt = [DateTime]::UtcNow.ToString('o'); artifacts = $artifactRows; resources = $resourceRows }
[IO.File]::WriteAllText((Join-Path $distDir 'adventure-elite-build.json'), ($buildIdentity | ConvertTo-Json -Depth 6), (New-Object Text.UTF8Encoding($false)))

<#
.SYNOPSIS
  编译 Go 启动器和服务端候选，校验程序身份，不启动游戏。
.DESCRIPTION
  先执行全量 build / vet / test，再分别构建 cmd/dfolauncher 和 cmd/wireprobe。
  产物都通过身份检查后，备份旧文件并安装启动器与源码候选。
  本机已有精锐专用启动器时一并重建，避免该入口继续运行更新前的 profile 校验。
  按本机已安装的 mod 生成临时 Go overlay，保留仓库加载器原文。
  完整基础测试使用干净加载器；另行核对本机 mod 的声明与注册，候选编译包含本机 mod。
  默认服务端 wireprobe-pvf.exe、39 归档、存档及客户端均保留。
  需要 Go 1.26；优先使用 PATH，也可使用便携 Go 或已缓存的 Go 1.26 工具链。
  退出码：0 成功；1 工具链、检查、构建或安装失败。
.EXAMPLE
  powershell -NoProfile -ExecutionPolicy Bypass -File scripts\build-game.ps1
.EXAMPLE
  powershell -NoProfile -ExecutionPolicy Bypass -File scripts\build-game.ps1 -Go D:\Go\bin\go.exe
#>
param([string]$Go)

$ErrorActionPreference = 'Stop'
$RepoRoot = Split-Path -Parent $PSScriptRoot
$ModuleRoot = Join-Path $RepoRoot 'server\work\dfo-lan'
$OriginalLocation = Get-Location
$SavedEnvironment = @{}
foreach ($key in @('GOROOT', 'GOTOOLCHAIN', 'GOPROXY', 'GOPATH', 'GOMODCACHE', 'GOCACHE')) {
    $SavedEnvironment[$key] = [Environment]::GetEnvironmentVariable($key, 'Process')
}

function New-ServerModBuildOverlay([string]$ModuleRoot, [string]$BuildDir, [switch]$Clean) {
    $modsDir = Join-Path $ModuleRoot 'mods'
    $modNames = @()
    if (-not $Clean -and (Test-Path -LiteralPath $modsDir -PathType Container)) {
        foreach ($directory in @(Get-ChildItem -LiteralPath $modsDir -Directory | Sort-Object Name)) {
            # 与 modkit 的安装标记一致；普通源码目录不自动变成已安装 mod。
            $owned = Test-Path -LiteralPath (Join-Path $directory.FullName '.modkit-owner') -PathType Leaf
            $manifest = Test-Path -LiteralPath (Join-Path $directory.FullName 'mod.json') -PathType Leaf
            if (-not $owned -and -not $manifest) { continue }
            $goSources = @(Get-ChildItem -LiteralPath $directory.FullName -Filter '*.go' -File |
                Where-Object { $_.Name -notlike '*_test.go' })
            if ($goSources.Count -eq 0) {
                if ($manifest) {
                    throw ('mod 安装不完整：{0} 存在 mod.json，但包目录没有 Go 源码；请通过 mod 管理器重新安装或完整卸载。' -f $directory.FullName)
                }
                Write-Host ('[跳过] 残留 mod 安装标记：{0} 没有 Go 源码或 mod.json，不加入编译清单。' -f $directory.FullName) -ForegroundColor Yellow
                continue
            }
            if ($directory.Name -notmatch '^[A-Za-z0-9][A-Za-z0-9_.-]*$') {
                throw ('已安装 mod 的目录名无法生成 Go import：{0}' -f $directory.Name)
            }
            $modNames += $directory.Name
        }
    }
    $lines = @('// Code generated for this local build. DO NOT EDIT.', 'package mods', '')
    if ($modNames.Count -gt 0) {
        $lines += 'import ('
        for ($index = 0; $index -lt $modNames.Count; $index++) {
            $lines += ('    mod{0} "dfolan/mods/{1}"' -f $index, $modNames[$index])
        }
        $lines += ')', ''
    }
    $lines += 'func RegisterMods() {'
    for ($index = 0; $index -lt $modNames.Count; $index++) {
        $lines += ('    mod{0}.Register()' -f $index)
    }
    $lines += '}', ''
    # overlay 读取内容不要求后缀为 .go；避免 go ./... 扫到 runtime 下的生成物。
    $name = if ($Clean) { 'mods-clean' } else { 'mods-installed' }
    $generated = Join-Path $BuildDir ($name + '.go.build')
    [IO.File]::WriteAllText($generated, ($lines -join "`n"), [Text.UTF8Encoding]::new($false))
    $replace = @{}
    $replace[[IO.Path]::GetFullPath((Join-Path $modsDir 'zz_mods_gen.go'))] = [IO.Path]::GetFullPath($generated)
    $overlay = Join-Path $BuildDir ($name + '-overlay.json')
    [IO.File]::WriteAllText($overlay, (@{ Replace = $replace } | ConvertTo-Json -Depth 4), [Text.UTF8Encoding]::new($false))
    if ($Clean) { Write-Host '完整基础测试：干净 mod 加载器（独立核对已安装 mod）。' }
    else { Write-Host ('本机 mod 编译清单（仓库加载器保持）：{0}' -f $(if ($modNames.Count) { $modNames -join ', ' } else { '无' })) }
    return $overlay
}

try {
    Set-Location -LiteralPath $RepoRoot
    # 使用本地工具链；已缓存的 Go 1.26 可直接运行。
    $env:GOTOOLCHAIN = 'local'
    Remove-Item Env:GOROOT -ErrorAction SilentlyContinue
    $goOnPath = Get-Command go.exe -ErrorAction SilentlyContinue
    $moduleCache = $SavedEnvironment['GOMODCACHE']
    if (-not $moduleCache -and $goOnPath) {
        $moduleCache = (& $goOnPath.Source env GOMODCACHE | Out-String).Trim()
        if ($LASTEXITCODE -ne 0) { throw '无法查询 Go 依赖缓存目录。' }
    }
    $candidates = @()
    if ($Go) {
        $candidates = @((Get-Command $Go -ErrorAction Stop).Source)
    } else {
        if ($goOnPath) { $candidates += $goOnPath.Source }
        $candidates += Join-Path (Split-Path -Parent $RepoRoot) 'tools\go\bin\go.exe'
        $candidates += 'C:\Game\dof\115us\tools\go\bin\go.exe'
        if ($moduleCache) {
            $cachedRoot = Join-Path $moduleCache 'golang.org'
            if (Test-Path -LiteralPath $cachedRoot) {
                $cached = @(Get-ChildItem -LiteralPath $cachedRoot -Directory -Filter 'toolchain@v0.0.1-go1.26*.windows-amd64' |
                    Sort-Object { [version](($_.Name -split '-go')[1] -split '.windows-')[0] } -Descending)
                foreach ($entry in $cached) { $candidates += Join-Path $entry.FullName 'bin\go.exe' }
            }
        }
    }
    $GoExe = $null
    foreach ($candidate in $candidates) {
        if (-not (Test-Path -LiteralPath $candidate -PathType Leaf)) { continue }
        $versionOutput = (& $candidate version | Out-String).Trim()
        if ($LASTEXITCODE -eq 0 -and $versionOutput -match '^go version go1\.26(?:\.\d+)? windows/amd64$') {
            $GoExe = $candidate
            Write-Host ('工具链：{0} ({1})' -f $GoExe, $versionOutput)
            break
        }
    }
    if (-not $GoExe) { throw '找不到 Go 1.26 Windows amd64 工具链。请安装 Go 1.26，或用 -Go 指定其 go.exe。' }

    $env:GOPROXY = 'https://goproxy.cn,direct'
    $env:GOPATH = 'C:\Game\dof\115us\tools\gopath'
    # 复用本机依赖缓存；不存在时只在忽略的 runtime 目录准备依赖。
    $env:GOMODCACHE = if ($moduleCache) { $moduleCache } else { Join-Path $ModuleRoot 'runtime\go-mod' }
    $env:GOCACHE = Join-Path $ModuleRoot 'runtime\gocache'
    $BuildDir = Join-Path $ModuleRoot ('runtime\build-game\' + (Get-Date -Format 'yyyyMMdd-HHmmss-fff'))
    New-Item -ItemType Directory -Path $BuildDir -ErrorAction Stop | Out-Null
    $LogPath = Join-Path $BuildDir 'build.log'
    $Utf8 = New-Object System.Text.UTF8Encoding($false)
    $ModOverlay = New-ServerModBuildOverlay -ModuleRoot $ModuleRoot -BuildDir $BuildDir
    $CleanOverlay = New-ServerModBuildOverlay -ModuleRoot $ModuleRoot -BuildDir $BuildDir -Clean
    # 单独进程核对与候选相同的加载器及运行启用名单，不访问存档或执行 boot 钩子。
    $registrationTest = @'
package mods_test
import (
    "testing"
    "dfolan/internal/servermod"
    "dfolan/mods"
)
func TestLocalBuildInstalledModDeclarations(t *testing.T) {
    servermod.LoadEnabledList(".")
    mods.RegisterMods()
    problems, err := servermod.CheckDeclarations(".")
    if err != nil { t.Fatal(err) }
    if len(problems) != 0 { t.Fatalf("installed mod declarations differ: %v", problems) }
    t.Logf("registered mods: %v", servermod.Registered())
}
'@
    $registrationSource = Join-Path $BuildDir 'registration_test.go.build'
    [IO.File]::WriteAllText($registrationSource, $registrationTest, $Utf8)
    $registrationReplace = @{}
    $installedData = [IO.File]::ReadAllText($ModOverlay) | ConvertFrom-Json
    foreach ($property in $installedData.Replace.PSObject.Properties) { $registrationReplace[$property.Name] = $property.Value }
    $registrationReplace[(Join-Path $ModuleRoot 'mods/build_installed_mods_test.go')] = $registrationSource
    $RegistrationOverlay = Join-Path $BuildDir 'mods-registration-overlay.json'
    [IO.File]::WriteAllText($RegistrationOverlay, (@{ Replace = $registrationReplace } | ConvertTo-Json -Depth 4), $Utf8)

    function Invoke-Go([string[]]$Arguments, [string]$Overlay = $ModOverlay) {
        $remaining = @($Arguments | Select-Object -Skip 1)
        $label = 'go ' + $Arguments[0] + ' -overlay="' + $Overlay + '" ' + ($remaining -join ' ')
        Write-Host $label -ForegroundColor Cyan
        [IO.File]::AppendAllText($LogPath, $label + "`r`n", $Utf8)
        # PS 5.1 会把原生 stderr 包装成 ErrorRecord；按 Go 退出码判断失败。
        $savedPreference = $ErrorActionPreference
        $ErrorActionPreference = 'Continue'
        try {
            & $GoExe $Arguments[0] "-overlay=$Overlay" @remaining 2>&1 | ForEach-Object {
                Write-Host $_
                [IO.File]::AppendAllText($LogPath, $_.ToString() + "`r`n", $Utf8)
            }
            $code = $LASTEXITCODE
        } finally { $ErrorActionPreference = $savedPreference }
        if ($code -ne 0) { throw ('{0} 失败，退出码 {1}。日志：{2}' -f $label, $code, $LogPath) }
    }

    Set-Location -LiteralPath $ModuleRoot
    Invoke-Go -Arguments @('build', './...')
    Invoke-Go -Arguments @('vet', './...')
    Invoke-Go -Arguments @('test', './...', '-count=1') -Overlay $CleanOverlay
    Invoke-Go -Arguments @('test', './mods', '-run', '^TestLocalBuildInstalledModDeclarations$', '-count=1', '-v') -Overlay $RegistrationOverlay
    $artifacts = @(
        @{ Name = 'dfolauncher.exe'; Package = './cmd/dfolauncher'; Identity = 'dfolan/cmd/dfolauncher' },
        @{ Name = 'wireprobe-handoff-source.exe'; Package = './cmd/wireprobe'; Identity = 'dfolan/cmd/wireprobe' }
    )
    # 精锐入口使用同一份 Go 源码，但有独立文件名；已有入口必须随源码一起更新。
    if (Test-Path -LiteralPath (Join-Path $ModuleRoot 'bin\dfolauncher-adventure-elite.exe') -PathType Leaf) {
        $artifacts += @{ Name = 'dfolauncher-adventure-elite.exe'; Package = './cmd/dfolauncher'; Identity = 'dfolan/cmd/dfolauncher' }
    }
    foreach ($artifact in $artifacts) {
        $builtPath = Join-Path $BuildDir $artifact.Name
        Invoke-Go -Arguments @('build', '-trimpath', '-o', $builtPath, $artifact.Package)
        $metadata = (& $GoExe version -m $builtPath | Out-String)
        if ($LASTEXITCODE -ne 0 -or $metadata -notmatch ('(?m)^\s*path\s+' + [regex]::Escape($artifact.Identity) + '\s*$')) {
            throw ('程序身份不匹配，停止安装：{0}，应为 {1}' -f $builtPath, $artifact.Identity)
        }
    }
    $BinDir = Join-Path $ModuleRoot 'bin'
    New-Item -ItemType Directory -Path $BinDir -Force | Out-Null
    foreach ($artifact in $artifacts) {
        $destination = Join-Path $BinDir $artifact.Name
        if (Test-Path -LiteralPath $destination) {
            Copy-Item -LiteralPath $destination -Destination (Join-Path $BuildDir ($artifact.Name + '.before'))
        }
        Copy-Item -LiteralPath (Join-Path $BuildDir $artifact.Name) -Destination $destination
        $item = Get-Item -LiteralPath $destination
        $hash = (Get-FileHash -LiteralPath $destination -Algorithm SHA256).Hash
        Write-Host ('已安装：{0} ({1} bytes)，SHA256={2}' -f $destination, $item.Length, $hash)
    }
    Write-Host ('构建完成；日志、产物与旧文件备份：{0}' -f $BuildDir)
    Write-Host '下一步：手动运行 scripts\启动游戏-SQLite.cmd --source-build 验证源码候选。'
    Write-Host '默认 wireprobe-pvf.exe 保留；候选验收通过后再按 server\Build-Server.ps1 -UpdatePVFDefault 发布。'
    $exitCode = 0
} catch {
    Write-Host ('编译失败：{0}' -f $_.Exception.Message) -ForegroundColor Red
    $exitCode = 1
} finally {
    Set-Location -LiteralPath $OriginalLocation.Path
    foreach ($key in $SavedEnvironment.Keys) {
        [Environment]::SetEnvironmentVariable($key, $SavedEnvironment[$key], 'Process')
    }
}
exit $exitCode

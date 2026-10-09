<#
.SYNOPSIS
  编译 Go 启动器和服务端候选，校验程序身份，不启动游戏。
.DESCRIPTION
  先执行全量 build / vet / test，再分别构建 cmd/dfolauncher 和 cmd/wireprobe。
  两个产物都通过身份检查后，备份旧文件并安装启动器与源码候选。
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

    function Invoke-Go([string[]]$Arguments) {
        $label = 'go ' + ($Arguments -join ' ')
        Write-Host $label -ForegroundColor Cyan
        [IO.File]::AppendAllText($LogPath, $label + "`r`n", $Utf8)
        # PS 5.1 会把原生 stderr 包装成 ErrorRecord；按 Go 退出码判断失败。
        $savedPreference = $ErrorActionPreference
        $ErrorActionPreference = 'Continue'
        try {
            & $GoExe @Arguments 2>&1 | ForEach-Object {
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
    Invoke-Go -Arguments @('test', './...', '-count=1')
    $artifacts = @(
        @{ Name = 'dfolauncher.exe'; Package = './cmd/dfolauncher'; Identity = 'dfolan/cmd/dfolauncher' },
        @{ Name = 'wireprobe-handoff-source.exe'; Package = './cmd/wireprobe'; Identity = 'dfolan/cmd/wireprobe' }
    )
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

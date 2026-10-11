<#
.SYNOPSIS
  验证编译入口的 mod 发现及残留安装标记，不启动游戏或操作真实 mod。
.NOTES
  只写本任务的临时夹具，不改仓库加载器；退出码 0 通过，1 失败。
#>
$ErrorActionPreference = 'Stop'
$RepoRoot = Split-Path -Parent $PSScriptRoot
Set-Location -LiteralPath $RepoRoot
$tokens = $null
$parseErrors = $null
$ast = [Management.Automation.Language.Parser]::ParseFile((Join-Path $PSScriptRoot 'build-game.ps1'), [ref]$tokens, [ref]$parseErrors)
if ($parseErrors.Count) { throw ($parseErrors | Out-String) }
$definitions = @($ast.FindAll({ param($node)
    $node -is [Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq 'New-ServerModBuildOverlay'
}, $true))
if ($definitions.Count -ne 1) { throw '无法找到唯一的 mod overlay 生成函数。' }
# Only execute the definition, never the build script's main body.
. ([ScriptBlock]::Create($definitions[0].Extent.Text))
$fixtureRoot = Join-Path $RepoRoot ('.tmp\build-game-mods-tests\' + [Guid]::NewGuid().ToString('N'))
$module = Join-Path $fixtureRoot 'module'
$build = Join-Path $fixtureRoot 'build'
$mods = Join-Path $module 'mods'
New-Item -ItemType Directory -Path $mods, $build | Out-Null
$utf8 = [Text.UTF8Encoding]::new($false)
$loader = Join-Path $mods 'zz_mods_gen.go'
[IO.File]::WriteAllText($loader, 'original loader', $utf8)

function Read-GeneratedLoader([switch]$Clean) {
    $overlay = New-ServerModBuildOverlay -ModuleRoot $module -BuildDir $build -Clean:$Clean
    $data = [IO.File]::ReadAllText($overlay) | ConvertFrom-Json
    $replacement = $data.Replace.PSObject.Properties | Where-Object { $_.Name -eq $loader }
    if (-not $replacement) { throw 'Overlay 未指向原加载器。' }
    return [IO.File]::ReadAllText($replacement.Value)
}

if ((Read-GeneratedLoader) -match 'import') { throw '空安装被导入。' }
foreach ($name in @('z-owned', 'a-manifest', 'orphan', 'unowned', 'test-only')) {
    New-Item -ItemType Directory -Path (Join-Path $mods $name) | Out-Null
}
foreach ($name in @('z-owned', 'orphan', 'test-only')) {
    [IO.File]::WriteAllText((Join-Path $mods ($name + '\.modkit-owner')), 'fixture', $utf8)
}
foreach ($name in @('z-owned', 'a-manifest', 'unowned')) {
    [IO.File]::WriteAllText((Join-Path $mods ($name + '\mod.go')), "package modpkg`nfunc Register() {}`n", $utf8)
}
[IO.File]::WriteAllText((Join-Path $mods 'a-manifest\mod.json'), '{}', $utf8)
[IO.File]::WriteAllText((Join-Path $mods 'test-only\mod_test.go'), 'package modpkg', $utf8)
$source = Read-GeneratedLoader
if ($source -notmatch 'mod0 "dfolan/mods/a-manifest"' -or $source -notmatch 'mod1 "dfolan/mods/z-owned"') { throw '安装发现或稳定排序错误。' }
if ($source -match 'orphan|unowned|test-only') { throw '残留标记、普通目录或仅测试源码被导入。' }
if (-not (Test-Path -LiteralPath (Join-Path $mods 'orphan\.modkit-owner'))) { throw '残留标记被删除。' }
if ((Read-GeneratedLoader -Clean) -match 'import') { throw '干净测试加载器包含本机 mod。' }
[IO.File]::WriteAllText((Join-Path $mods 'orphan\mod.json'), '{}', $utf8)
$rejected = $false
try { $null = Read-GeneratedLoader } catch {
    if ($_.Exception.Message -notmatch 'mod 安装不完整') { throw }
    $rejected = $true
}
if (-not $rejected) { throw '仍有清单却缺源码的安装未被拒绝。' }
if ([IO.File]::ReadAllText($loader) -ne 'original loader') { throw '仓库加载器被覆盖。' }
Write-Host 'PASS: empty, installed, sorted, orphan marker, test-only, unowned, clean overlay, incomplete manifest, original files preserved.'

# 拼装 gopath-mod 模块缓存包（分片 → 单个 zip）
#
# 背景：`tools/tools-gopath-mod.zip` 约 62.7 MB，单次推送会被远端断开
# （`send-pack: unexpected disconnect while reading sideband packet`）。
# 于是仓库里存的是分片 `tools/tools-gopath-mod.zip.partNN`（每片 16 MiB），
# 本脚本按 manifest 记录的大小/哈希把它们拼回原文件。
#
# 用法：
#   pwsh -NoProfile -File scripts/assemble-gopath-mod.ps1
#   powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts\assemble-gopath-mod.ps1
#
# 幂等：目标已存在且 size+sha256 与 manifest 一致时直接返回 0，不重写。
# 退出码：0 成功（已就绪或已拼好）；2 分片缺失/读不动；3 拼出的文件与 manifest 不一致；1 脚本自身错误。
#
# 只读性：除写入/覆盖 `tools/tools-gopath-mod.zip` 外不修改任何文件。

param(
    [string]$RepoRoot = '',
    [switch]$Force
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

if ([string]::IsNullOrWhiteSpace($RepoRoot)) {
    # 本脚本位于 <repo>/scripts/，仓库根 = 上一层。
    $RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
}
$toolsDir   = Join-Path $RepoRoot 'tools'
$manifest   = Join-Path $toolsDir 'manifest.json'
$target     = Join-Path $toolsDir 'tools-gopath-mod.zip'
$partPrefix = $target + '.part'

function Get-Sha256([string]$Path) {
    return (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLower()
}

if (-not (Test-Path -LiteralPath $manifest)) { throw "找不到 manifest：$manifest" }
$doc = Get-Content -LiteralPath $manifest -Raw -Encoding UTF8 | ConvertFrom-Json
$entry = @($doc.packages | Where-Object { $_.name -eq 'gopath-mod' })[0]
if (-not $entry) { throw 'manifest 里没有 gopath-mod 条目' }
$wantSize = [long]$entry.size
$wantSha  = ([string]$entry.sha256).ToLower()

if ((Test-Path -LiteralPath $target) -and -not $Force) {
    $haveSize = (Get-Item -LiteralPath $target).Length
    if ($haveSize -eq $wantSize -and (Get-Sha256 $target) -eq $wantSha) {
        Write-Output ("gopath-mod 已就绪（size={0} sha256={1}），无需拼装。" -f $haveSize, $wantSha.Substring(0, 16))
        exit 0
    }
    Write-Output ("现有 {0} 与 manifest 不一致（size={1}，期望 {2}）：重新拼装。" -f (Split-Path $target -Leaf), $haveSize, $wantSize)
}

$parts = @(Get-ChildItem -LiteralPath $toolsDir -File -ErrorAction SilentlyContinue |
    Where-Object { $_.Name -like ((Split-Path $target -Leaf) + '.part*') } |
    Sort-Object Name)
if ($parts.Count -eq 0) {
    Write-Output ("找不到分片：{0}*（期望至少 1 片）" -f $partPrefix)
    exit 2
}

Write-Output ("拼装 {0} 片 → {1}" -f $parts.Count, (Split-Path $target -Leaf))
$tmp = $target + '.assembling'
if (Test-Path -LiteralPath $tmp) { Remove-Item -LiteralPath $tmp -Force }
$out = [IO.File]::Create($tmp)
try {
    foreach ($p in $parts) {
        $bytes = [IO.File]::ReadAllBytes($p.FullName)
        $out.Write($bytes, 0, $bytes.Length)
        Write-Output ("  + {0}  {1:N0} B" -f $p.Name, $bytes.Length)
    }
} finally {
    $out.Close()
}

$gotSize = (Get-Item -LiteralPath $tmp).Length
$gotSha  = Get-Sha256 $tmp
if ($gotSize -ne $wantSize -or $gotSha -ne $wantSha) {
    Remove-Item -LiteralPath $tmp -Force
    Write-Output ("拼装结果与 manifest 不一致：size={0}（期望 {1}）sha256={2}（期望 {3}）" -f `
        $gotSize, $wantSize, $gotSha.Substring(0, 16), $wantSha.Substring(0, 16))
    exit 3
}

Move-Item -LiteralPath $tmp -Destination $target -Force
Write-Output ("已就绪：{0}  size={1}  sha256={2}" -f $target, $gotSize, $gotSha)
exit 0

param([string]$Go = 'go', [switch]$UpdatePVFDefault, [switch]$CheckSQL, [switch]$SkipEnvEnsure, [switch]$SkipModGenGate)
$ErrorActionPreference = 'Stop'

# 编译环境自举（业主 2026-10-05 指示：编译时自动触发，如果不存在整包就触发）。
#
# 背景：依赖缓存包 tools/tools-gopath-mod.zip 约 62.7 MB，单次推送会被远端断开
# （send-pack: unexpected disconnect），所以仓库里只存分片 tools/tools-gopath-mod.zip.partNN。
# 这里在编译前检查：整包不在、但分片在 → 先调 scripts/assemble-gopath-mod.ps1 拼回，
# 该脚本会按 tools/manifest.json 的 size/sha256 校验，幂等（已就绪则零写入）。
#
# -SkipEnvEnsure 只给测试/排查用：跳过这段，直接编译。
$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$PackPath = Join-Path $RepoRoot 'tools\tools-gopath-mod.zip'
$PartGlob = 'tools-gopath-mod.zip.part*'

if (-not $SkipEnvEnsure -and -not (Test-Path -LiteralPath $PackPath)) {
    $parts = @(Get-ChildItem -LiteralPath (Join-Path $RepoRoot 'tools') -File -Filter $PartGlob -ErrorAction SilentlyContinue)
    if ($parts.Count -gt 0) {
        Write-Output ("依赖缓存整包不存在，发现 {0} 个分片：先拼装…" -f $parts.Count)
        & (Join-Path $RepoRoot 'scripts\assemble-gopath-mod.ps1') -RepoRoot $RepoRoot
        if ($LASTEXITCODE -ne 0) { throw ("拼装 tools-gopath-mod.zip 失败（exit {0}）" -f $LASTEXITCODE) }
    } else {
        Write-Output '依赖缓存整包与分片都不在：跳过拼装（若编译失败，请按 tools/manifest.json 取 gopath-mod 包）。'
    }
}

# mod 加载器门禁（2026-10-07）：挡住"加载器 import 了只在本机的 mod"。
#
# 背景：mods/zz_mods_gen.go 是 modkit 生成的加载器，写的是**本机**装了哪些 mod；这份文件
# 同时进版本库。本机装了别人没有的 mod 又被推上去，别人全新 clone 直接编不过：
#
#   mods\zz_mods_gen.go:9:2: package dfolan/mods/<mod-id> is not in std
#
# （模块里找不到该导入路径时，Go 会把它当标准库候选去 GOROOT 找，报错因此长得不像"缺文件"。）
# 判据：加载器里 import 的每个 dfolan/mods/<id>，其源码都必须能被 git 找到（随仓库走）。
# 有未入库的 import → 明确报错并中止编译；没有可用 git（玩家的解包目录）→ 只告警不阻断。
# -SkipModGenGate 只给"本机确实要带着已装 mod 编译"的场合用（那时加载器本来就该是本机形态）。
if (-not $SkipModGenGate) {
    $ModsGenPath = Join-Path $RepoRoot 'server\work\dfo-lan\mods\zz_mods_gen.go'
    if (Test-Path -LiteralPath $ModsGenPath) {
        $ModsImports = @()
        $ModsGenText = Get-Content -LiteralPath $ModsGenPath -Raw -Encoding UTF8
        foreach ($ModsMatch in [regex]::Matches($ModsGenText, '"dfolan/mods/([^"]+)"')) {
            $ModsImports += $ModsMatch.Groups[1].Value
        }
        if ($ModsImports.Count -eq 0) {
            Write-Output 'mod 加载器门禁：zz_mods_gen.go 是干净形态（没有 import 任何 mod），通过。'
        } else {
            # 能不能用 git 核对"是否入库"？用一条已知入库的文件自证 git 可用；
            # 不可用（没有 git、不是工作区、解包目录）时退化成只告警，绝不阻断玩家的编译。
            $ModsGitUsable = $false
            if ((Get-Command git -ErrorAction SilentlyContinue) -and (Test-Path -LiteralPath (Join-Path $RepoRoot '.git'))) {
                $ModsProbe = @(& git -C $RepoRoot ls-files -- 'server/work/dfo-lan/mods/zz_mods_gen.go')
                $ModsGitUsable = ($LASTEXITCODE -eq 0 -and $ModsProbe.Count -gt 0)
            }
            if (-not $ModsGitUsable) {
                Write-Output ('mod 加载器门禁：加载器 import 了 {0} 个 mod（{1}），但这里没有可用的 git 仓库，无法核对它们是否入库——只告警，不阻断编译。' -f $ModsImports.Count, ($ModsImports -join '、'))
            } else {
                $ModsMissing = @()
                foreach ($ModsID in $ModsImports) {
                    $ModsTracked = @(& git -C $RepoRoot ls-files -- ('server/work/dfo-lan/mods/' + $ModsID))
                    if ($LASTEXITCODE -ne 0 -or $ModsTracked.Count -eq 0) { $ModsMissing += $ModsID }
                }
                if ($ModsMissing.Count -eq 0) {
                    Write-Output ('mod 加载器门禁：加载器 import 的 {0} 个 mod 都已入库（{1}），通过。' -f $ModsImports.Count, ($ModsImports -join '、'))
                } else {
                    Write-Output '----------------------------------------------------------------'
                    Write-Output 'mod 加载器门禁：不通过，编译已中止'
                    foreach ($ModsID in $ModsMissing) {
                        Write-Output ('  未入库的 mod：{0}' -f $ModsID)
                        Write-Output ('    server/work/dfo-lan/mods/zz_mods_gen.go 里 import 了 dfolan/mods/{0}，' -f $ModsID)
                        Write-Output ('    但 git 找不到它的源码（git ls-files --error-unmatch server/work/dfo-lan/mods/{0} 无匹配）。' -f $ModsID)
                        Write-Output  '    为什么不行：它只在你本机（modkit 落位、未跟踪），别人 clone 下来没有这个目录，'
                        Write-Output  '    服务端一编就断：package dfolan/mods/<mod-id> is not in std。'
                    }
                    Write-Output '两条出路（任选其一）：'
                    Write-Output '  1) 把该 mod 的源码入库：server/work/dfo-lan/mods/<mod-id>/ 一起提交，再提交加载器；'
                    Write-Output '  2) 先清空加载器 / 在启动器里卸载该 mod（modkit uninstall --id <mod-id>），再提交干净形态。'
                    Write-Output '说明：本机带着已装 mod 编译（不提交）时加 -SkipModGenGate 跳过这道门禁；'
                    Write-Output '      启动器自己的编译路径不受本门禁影响（它编译前会按本机 mods/ 现状重算加载器）。'
                    Write-Output '详见 server/work/dfo-lan/mods/README.md §加载器 zz_mods_gen.go。'
                    Write-Output '----------------------------------------------------------------'
                    throw ('mod 加载器门禁未过：未入库的 import：{0}' -f ($ModsMissing -join '、'))
                }
            }
        }
    }
}

Push-Location (Join-Path $PSScriptRoot 'work/dfo-lan')
try {
    # Normal builds use checked-in generated Go. SQL authors opt into the
    # pinned generator check; this does not require a database connection.
    if ($CheckSQL) {
        & ./scripts/Generate-SQL.ps1 -Check
    }
    & $Go test ./...
    if ($LASTEXITCODE -ne 0) { throw 'Tests failed' }
    & $Go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'Vet failed' }
    & $Go build -trimpath -o bin/wireprobe-handoff-source.exe ./cmd/wireprobe
    if ($LASTEXITCODE -ne 0) { throw 'Build failed' }
    if ($UpdatePVFDefault -or -not (Test-Path -LiteralPath bin/wireprobe-pvf.exe)) {
        Copy-Item -LiteralPath bin/wireprobe-handoff-source.exe -Destination bin/wireprobe-pvf.exe -Force
        Write-Output 'Built source and default PVF gateway; archived39 was preserved.'
    } else {
        Write-Output 'Built source candidate; confirmed PVF gateway was preserved. Use -UpdatePVFDefault to publish an accepted build.'
    }
} finally { Pop-Location }

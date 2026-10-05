<#
.SYNOPSIS
  DFO 115us 提交前门禁：检查「本地缓存/构建产物入库」与「AGENTS §0.4/§0.5 规范违规」。

.DESCRIPTION
  只读脚本：只查询 git 与文件系统，不修改任何文件、不动暂存区。

  三类校验：
    1. 缓存/本地产物与目录规范（§0.3.1 / §0.4）——runtime、pvf-cache、日志、库文件、
       根目录 .cmd、新增顶层文件、新增 cmd/<非白名单>、新增服务端 configs JSON 等；
    2. 二进制与大文件（§0.3.3）—— 新增二进制/压缩包、新增 >5MB blob；
    3. 环境匹配（§0.3.4）—— 提交内容与「本机正在跑的这套环境」是否对得上：
       存储档 driver/sqlite_path、示例档引用路径、profile 指向的程序、
       启动链引用的频道档、脚本引用的仓库内 tools\（本机已在仓库外）、
       .ps1 的 UTF-8 BOM 与 .cmd 的 CRLF。

  退出码：
    0 = 通过（可以直接提交）
    2 = 需要二次确认（发现缓存产物、规范违规或环境不匹配）
    1 = 脚本自身错误（不在 git 仓库、git 不可用等）

  一旦返回 2，按根 AGENTS.md §0.3.1 执行：
    立即停止提交 → 逐条报告业主（路径/类别/条款/影响/建议）→ 取得业主【明确】的二次确认
    → 例外条目写进提交信息正文。禁止用 -Force / --no-verify / git add -A 绕过。

.PARAMETER All
  检查工作树全部改动（已暂存 + 未暂存 + 未跟踪）。默认只检查已暂存内容。

.PARAMETER Json
  以 JSON 输出结果（便于工具消费）。

.PARAMETER Quiet
  只输出结论行。

.EXAMPLE
  pwsh -NoProfile -File scripts/check-commit-hygiene.ps1
  pwsh -NoProfile -File scripts/check-commit-hygiene.ps1 -All
#>
[CmdletBinding()]
param(
    [switch]$All,
    [switch]$Json,
    [switch]$Quiet
)

$ErrorActionPreference = 'Stop'

# 仓库根 = 本脚本所在目录的上一级（scripts\ 在仓库根下；见根 AGENTS §0.4.2）
$RepoRoot = Split-Path -Parent $PSScriptRoot
if (-not (Test-Path -LiteralPath (Join-Path $RepoRoot '.git'))) {
    Write-Error "找不到仓库根（$RepoRoot 下没有 .git）。"
    exit 1
}
Push-Location $RepoRoot
try {
    if (-not (Get-Command git -ErrorAction SilentlyContinue)) { Write-Error 'git 不可用。'; exit 1 }

    $MAX_NEW_BLOB_MB = 5.0          # 新增单个 blob 超过它就要征询业主（§0.3.3）
    $ALLOWED_CMD_ENTRIES = @('wireprobe', 'admin', 'gmtool', 'dfo-tool')   # §0.4.1 只允许这四个入口

    # ---- 1. 缓存 / 本地生成物：一律阻断 ----
    $denyPatterns = @(
        @{ Re = '(^|/)runtime/';                 Why = 'runtime 是运行期持久化状态（会话日志/库文件），严禁入库' }
        @{ Re = '(^|/)pvf-cache/';               Why = 'PVF 派生缓存，可由源重建' }
        @{ Re = '(^|/)\.tmp/';                   Why = '临时工作目录' }
        @{ Re = '(^|/)__pycache__/';             Why = 'Python 字节码缓存' }
        @{ Re = '\.pyc$';                        Why = 'Python 字节码缓存' }
        @{ Re = '^node_modules/';                Why = '外部依赖' }
        @{ Re = '^dist/';                        Why = '构建产物' }
        @{ Re = '^build/';                       Why = '构建产物' }
        @{ Re = '\.log$';                        Why = '运行日志' }
        @{ Re = '(^|/)pgdata';                   Why = 'PostgreSQL 数据集群' }
        @{ Re = '\.sqlite3';                     Why = 'SQLite 存档/测试库文件' }
        @{ Re = '-wal$';                         Why = 'SQLite WAL' }
        @{ Re = '-shm$';                         Why = 'SQLite 共享内存' }
        @{ Re = 'admin-guard$';                  Why = '存储管理租约文件' }
        @{ Re = '\.first-try$';                  Why = '手工备份的库文件' }
        @{ Re = '\.orig-bak$';                   Why = '手工备份' }
        @{ Re = '\.dmp$';                        Why = '崩溃转储' }
        @{ Re = 'CrashDNF.*\.cra$';              Why = '客户端崩溃计数' }
        @{ Re = '\.(trc|npk|pvf)$';              Why = '客户端资源/归档（体积大且非源码）' }
        @{ Re = '(^|/)\.pytest_cache/';          Why = '测试缓存' }
        @{ Re = '(^|/)\.dsh-';                   Why = 'AI 会话临时产物' }
    )

    # ---- 2. 需要业主确认的类别（不阻断，但要二次确认）----
    $confirmPatterns = @(
        @{ Re = '\.(exe|dll|aes|bin|zip|7z|rar)$'; Why = '新增二进制/压缩包，需确认是否该入库（§0.3.3）' }
    )

    # ---- 3. 收集候选（路径 + 变更类型）----
    $items = @()
    if ($All) {
        foreach ($line in (git status --porcelain)) {
            if (-not $line.Trim()) { continue }
            $code = $line.Substring(0, 2)
            $path = $line.Substring(3).Trim().Trim('"')
            if ($path -match ' -> ') { $path = ($path -split ' -> ')[-1].Trim().Trim('"') }
            $kind = 'M'
            if ($code -match '\?\?') { $kind = 'A' }
            elseif ($code -match 'A') { $kind = 'A' }
            elseif ($code -match 'R') { $kind = 'R' }
            $items += [pscustomobject]@{ Path = $path; Kind = $kind }
        }
    }
    else {
        foreach ($line in (git diff --cached --name-status)) {
            if (-not $line.Trim()) { continue }
            $parts = $line -split "`t"
            if ($parts.Count -lt 2) { continue }
            $kind = $parts[0].Substring(0, 1)
            $path = $parts[-1].Trim().Trim('"')
            $items += [pscustomobject]@{ Path = $path; Kind = $kind }
        }
    }
    $items = $items | Sort-Object Path -Unique

    # 已被 git 跟踪的路径：修改它们属于正常改动，不是「把运行期产物入库」。
    # （历史遗留被跟踪的 runtime 产物例如 pgdata/postgresql.conf 仍会提示，但不阻断。）
    # 必须用 core.quotePath=false：默认输出会把非 ASCII 路径转义并加引号，取不到原始路径。
    $tracked = @{}
    foreach ($t in (git -c core.quotePath=false ls-files)) { if ($t) { $tracked[$t.Trim()] = $true } }

    # ---- 4. 逐条判定 ----
    $blocked = @(); $needConfirm = @(); $notes = @()
    foreach ($it in $items) {
        $p = $it.Path.Replace('\', '/')
        $isNew = ($it.Kind -eq 'A' -or $it.Kind -eq 'R')
        $isTracked = $tracked.ContainsKey($it.Path)

        foreach ($rule in $denyPatterns) {
            if ($p -match $rule.Re) {
                if ($isTracked) {
                    $notes += [pscustomobject]@{ Path = $it.Path; Why = "$($rule.Why)；该路径已被跟踪，建议 git rm --cached（或补 .gitignore）后另行提交"; Clause = '§0.3.1 缓存/本地产物' }
                }
                else {
                    $blocked += [pscustomobject]@{ Path = $it.Path; Why = $rule.Why; Clause = '§0.3.1 缓存/本地产物' }
                }
                break
            }
        }

        if ($isNew) {
            foreach ($rule in $confirmPatterns) {
                if ($p -match $rule.Re) {
                    $needConfirm += [pscustomobject]@{ Path = $it.Path; Why = $rule.Why; Clause = '§0.3.3 二进制/大文件' }
                    break
                }
            }
            # 根目录不得新增 .cmd（§0.4.1）
            if ($p -notmatch '/' -and $p -match '\.cmd$') {
                $blocked += [pscustomobject]@{ Path = $it.Path; Why = '根目录不得有 .cmd，运行脚本一律放 scripts\'; Clause = '§0.4.1 目录落位' }
            }
            # 新增顶层文件（不在白名单）需要确认（§0.4.3）
            elseif ($p -notmatch '/') {
                $allow = @('AGENTS.md', 'CHANGELOG', 'README.md', '开发对接文档.md', '使用教程.md', '.gitignore', '.gitattributes')
                $ok = ($allow -contains $p) -or ($p -like 'MERGE-RECORD-*.md')
                if (-not $ok) {
                    $needConfirm += [pscustomobject]@{ Path = $it.Path; Why = '新增顶层文件，根目录只保留白名单内容'; Clause = '§0.4.3 命名与落位' }
                }
            }
            # 服务端新增 cmd/<工具名> 目录（§0.4.1）
            if ($p -match '^server/work/dfo-lan/cmd/([^/]+)/') {
                if ($ALLOWED_CMD_ENTRIES -notcontains $Matches[1]) {
                    $blocked += [pscustomobject]@{ Path = $it.Path; Why = "新增 cmd/$($Matches[1]) 入口；工具应进 internal/toolcmd/<name> 并由 cmd/dfo-tool 统一调用"; Clause = '§0.4.1 Go 落位' }
                }
            }
            # 服务端新增 configs/*.json（§0.4.1 + §0 铁律）
            if ($p -match '^server/work/dfo-lan/configs/.+\.json$') {
                $needConfirm += [pscustomobject]@{ Path = $it.Path; Why = '新增服务端 configs JSON，可能违反「单一内容真源」；只允许历史基线/策略并需显式说明'; Clause = '§0.4.1 / §0 铁律 1-3' }
            }
        }

        # 体积：新增 blob > 阈值
        if ($isNew) {
            $bytes = 0
            try {
                if ($it.Kind -eq 'A' -and -not $All) {
                    $bytes = [int64](git cat-file -s ":$($it.Path)" 2>$null)
                }
                elseif (Test-Path -LiteralPath $it.Path) {
                    $bytes = (Get-Item -LiteralPath $it.Path).Length
                }
            }
            catch { $bytes = 0 }
            if ($bytes -gt ($MAX_NEW_BLOB_MB * 1MB)) {
                $mb = [math]::Round($bytes / 1MB, 1)
                $needConfirm += [pscustomobject]@{ Path = $it.Path; Why = ("新增文件 $mb MB，超过 $MAX_NEW_BLOB_MB MB 阈值"); Clause = '§0.3.3 二进制/大文件' }
            }
        }

        # ---- 环境匹配（§0.3.4）：脚本的编码/行尾必须匹配本机解释器 ----
        if ($p -match '\.(ps1|cmd)$') {
            $scriptPath = Join-Path $RepoRoot ($it.Path -replace '/', '\')
            if (Test-Path -LiteralPath $scriptPath) {
                $raw = [System.IO.File]::ReadAllBytes($scriptPath)
                if ($p -match '\.ps1$') {
                    $hasBom = ($raw.Length -ge 3 -and $raw[0] -eq 0xEF -and $raw[1] -eq 0xBB -and $raw[2] -eq 0xBF)
                    if (-not $hasBom) {
                        $blocked += [pscustomobject]@{ Path = $it.Path; Why = '本机 Windows PowerShell 5.1 对无 BOM 的 .ps1 按 ANSI(GBK) 解码，中文注释会直接导致语法错误（2026-10-04 实测）'; Clause = '§0.3.4 环境匹配' }
                    }
                }
                else {
                    $crlf = 0; $lone = 0
                    for ($i = 0; $i -lt $raw.Length; $i++) {
                        if ($raw[$i] -eq 10) { if ($i -gt 0 -and $raw[$i - 1] -eq 13) { $crlf++ } else { $lone++ } }
                    }
                    if ($crlf -eq 0 -and $lone -gt 0) {
                        $needConfirm += [pscustomobject]@{ Path = $it.Path; Why = ("本机 .cmd 约定 CRLF，此文件有 $lone 行是裸 LF"); Clause = '§0.3.4 环境匹配' }
                    }
                }
            }
        }
    }

    # ---- 4b. 环境匹配（§0.3.4）：提交内容与「本机正在跑的这一套环境」是否对得上 ----
    # 只报事实对不上（路径不存在 / 档位不一致），不猜意图。
    # (a) 存储档：活动档引用的路径必须在本机存在；与已跟踪示例档 driver 不一致时，
    #     若该 driver 有自己的路线档（双库双路线，见 scripts\存储档.ps1）则只提示。
    $activeCfg = Join-Path $RepoRoot 'server\work\dfo-lan\runtime\storage\local.json'
    $exampleCfg = Join-Path $RepoRoot 'server\work\dfo-lan\runtime\storage\local.example.json'
    if (Test-Path -LiteralPath $activeCfg) {
        try {
            $act = Get-Content -LiteralPath $activeCfg -Raw -Encoding UTF8 | ConvertFrom-Json
            $actDriver = if ($act.driver) { [string]$act.driver } elseif ($act.postgres_dsn) { 'postgres' } elseif ($act.sqlite_path) { 'sqlite' } else { 'postgres' }
            if ($actDriver -eq 'sqlite' -and $act.sqlite_path) {
                $liveDb = [string]$act.sqlite_path
                if (-not (Test-Path -LiteralPath $liveDb)) {
                    $needConfirm += [pscustomobject]@{ Path = 'server/work/dfo-lan/runtime/storage/local.json'; Why = "本机存储档 driver=sqlite，但 sqlite_path=$liveDb 不存在（当前环境起不来）"; Clause = '§0.3.4 环境匹配' }
                }
            }
            # 没有 driver 却同时写了 postgres_dsn 与 sqlite_path：引擎选择是「DSN 优先」，
            # 但两份配置混在一个文件里，最容易被读成另一个库（2026-10-05 pgsql 端无法登录）。
            if (-not $act.driver -and $act.postgres_dsn -and $act.sqlite_path) {
                $needConfirm += [pscustomobject]@{ Path = 'server/work/dfo-lan/runtime/storage/local.json'; Why = '同时写了 postgres_dsn 与 sqlite_path 却没有 driver（按唯一规则选 PostgreSQL；建议显式写 driver 或删掉不用的那个键）'; Clause = '§0.3.4 环境匹配' }
            }
            if ($act.driver -and $act.driver -eq 'postgres' -and $act.sqlite_path) {
                $needConfirm += [pscustomobject]@{ Path = 'server/work/dfo-lan/runtime/storage/local.json'; Why = 'driver=postgres 但档里还留着 SQLite 专有键 sqlite_path（配置环境会清理；手工改的建议删掉）'; Clause = '§0.3.4 环境匹配' }
            }
        }
        catch {
            # 读不动（含非法 JSON / BOM 之外的问题）本身就是环境不匹配，要报出来而不是吞掉。
            $needConfirm += [pscustomobject]@{ Path = 'server/work/dfo-lan/runtime/storage/local.json'; Why = "本机存储档无法解析（$($_.Exception.Message)）；服务端会因此启动失败"; Clause = '§0.3.4 环境匹配' }
        }
    }
    if (Test-Path -LiteralPath $exampleCfg) {
        try {
            $exa = Get-Content -LiteralPath $exampleCfg -Raw -Encoding UTF8 | ConvertFrom-Json
            $exaDriver = if ($exa.driver) { [string]$exa.driver } elseif ($exa.sqlite_path) { 'sqlite' } else { 'postgres' }
            if ((Test-Path -LiteralPath $activeCfg) -and $actDriver -and ($exaDriver -ne $actDriver)) {
                # 双库双路线（2026-10-05 业主定调）：本机可以合法地停在任一条路线上，示例档只对应其中一条。
                # 路线的证据是 scripts\存储档.ps1 管的那份档：local.<活动 driver>.json 存在 ⇒ 这是切换后的
                # 正常状态，只提示；连它都不存在，才说明活动档既不是示例档、也不是任何一条已建好的路线。
                $routeFile = Join-Path $RepoRoot ("server\work\dfo-lan\runtime\storage\local.{0}.json" -f $actDriver)
                if (Test-Path -LiteralPath $routeFile) {
                    $notes += [pscustomobject]@{ Path = 'server/work/dfo-lan/runtime/storage/local.json'; Why = "本机停在 $actDriver 路线（$(Split-Path -Leaf $routeFile) 存在）；已跟踪示例档 driver=$exaDriver 对应另一条路线"; Clause = '§0.3.4 环境匹配' }
                }
                else {
                    $needConfirm += [pscustomobject]@{ Path = 'server/work/dfo-lan/runtime/storage/local.example.json'; Why = "示例档 driver=$exaDriver，本机活动档 driver=$actDriver，且没有对应的路线档（照示例档配环境会得到另一套存储）"; Clause = '§0.3.4 环境匹配' }
                }
            }
            foreach ($field in @('postgres_bin', 'postgres_data')) {
                $v = [string]$exa.$field
                if ($v -and -not [System.IO.Path]::IsPathRooted($v)) {
                    if (-not (Test-Path -LiteralPath (Join-Path $RepoRoot ($v -replace '/', '\')))) {
                        $needConfirm += [pscustomobject]@{ Path = 'server/work/dfo-lan/runtime/storage/local.example.json'; Why = "$field=$v 在本机不存在（本环境的 tools 已移出仓库，该示例档已过时）"; Clause = '§0.3.4 环境匹配' }
                    }
                }
            }
        }
        catch { }
    }
    # (b) profile / 启动器指向的程序必须存在
    foreach ($entry in @(
            @{ Cfg = 'server\work\dfo-lan\configs\pvf-default.json'; Base = 'server\work\dfo-lan'; Field = 'binary' },
            @{ Cfg = 'server\launcher.local.json'; Base = 'server'; Field = 'server_binary' })) {
        $cfgPath = Join-Path $RepoRoot $entry.Cfg
        if (-not (Test-Path -LiteralPath $cfgPath)) { continue }
        try {
            $cfg = Get-Content -LiteralPath $cfgPath -Raw -Encoding UTF8 | ConvertFrom-Json
            $target = [string]$cfg.($entry.Field)
            if (-not $target) { continue }
            $resolved = Join-Path (Join-Path $RepoRoot $entry.Base) ($target -replace '/', '\')
            if (-not (Test-Path -LiteralPath $resolved)) {
                $needConfirm += [pscustomobject]@{ Path = $entry.Cfg; Why = "$($entry.Field)=$target 在本机不存在（启动会找不到程序）"; Clause = '§0.3.4 环境匹配' }
            }
        }
        catch { }
    }
    # (c) 启动链引用的频道配置档必须存在
    $probePath = Join-Path $RepoRoot 'server\work\dfo_probe_tools\channel_probe.py'
    if (Test-Path -LiteralPath $probePath) {
        $probeText = Get-Content -LiteralPath $probePath -Raw -Encoding UTF8
        foreach ($m in [regex]::Matches($probeText, 'channel\.local\d+\.json')) {
            if (-not (Test-Path -LiteralPath (Join-Path $RepoRoot ('server\work\dfo-lan\configs\' + $m.Value)))) {
                $needConfirm += [pscustomobject]@{ Path = 'server/work/dfo_probe_tools/channel_probe.py'; Why = "启动链引用的 configs/$($m.Value) 不存在（该档位启动会失败）"; Clause = '§0.3.4 环境匹配' }
            }
        }
    }
    # (d) 改动的脚本若引用「仓库内 tools\」——本机 tools 已移出仓库，该引用在当前环境不可解析
    if (-not (Test-Path -LiteralPath (Join-Path $RepoRoot 'tools'))) {
        foreach ($it in $items) {
            if ($it.Path -notmatch '\.(cmd|ps1)$') { continue }
            $scriptPath = Join-Path $RepoRoot ($it.Path -replace '/', '\')
            if (-not (Test-Path -LiteralPath $scriptPath)) { continue }
            $text = Get-Content -LiteralPath $scriptPath -Raw -Encoding UTF8
            if ($text -match '(?<!\.\.\\)tools\\(go|python)\\') {
                $needConfirm += [pscustomobject]@{ Path = $it.Path; Why = '引用了仓库内 tools\，但本机 tools 已移到仓库外（该分支在当前环境不可用）'; Clause = '§0.3.4 环境匹配' }
            }
        }
    }
    $blocked = @($blocked | Sort-Object Path, Why -Unique)
    $needConfirm = @($needConfirm | Sort-Object Path, Why -Unique)

    # ---- 5. 输出 ----
    $scope = if ($All) { '工作树全部改动（含未跟踪）' } else { '已暂存内容' }
    # @() 不能省：只有 1 个路径时 `$items` 是单个对象，PS 5.1 上 `$obj.Count` 为 $null，
    # 于是「共  个路径」这种空计数会出现在最小的那次提交上——正好是最需要看清范围的时候。
    $pathCount = @($items).Count
    if ($Json) {
        [pscustomobject]@{
            scope       = $scope
            paths       = $pathCount
            blocked     = $blocked
            needConfirm = $needConfirm
            notes       = $notes
            verdict     = if ($blocked.Count -or $needConfirm.Count) { 'confirm-required' } else { 'pass' }
        } | ConvertTo-Json -Depth 5
    }
    else {
        Write-Host ''
        Write-Host 'DFO 115us · 提交前门禁（check-commit-hygiene）' -ForegroundColor Cyan
        Write-Host ("范围：{0}，共 {1} 个路径" -f $scope, $pathCount)
        $envIssues = @($blocked + $needConfirm | Where-Object { $_.Clause -match '环境匹配' })
        $blockedOther = @($blocked | Where-Object { $_.Clause -notmatch '环境匹配' })
        $needOther = @($needConfirm | Where-Object { $_.Clause -notmatch '环境匹配' })
        if ($blockedOther.Count) {
            Write-Host ''
            Write-Host ("[违规] 缓存/本地产物或目录规范（{0}）" -f $blockedOther.Count) -ForegroundColor Red
            foreach ($b in $blockedOther) { Write-Host ("   - {0}`n       {1}  [{2}]" -f $b.Path, $b.Why, $b.Clause) -ForegroundColor Red }
        }
        if ($envIssues.Count) {
            Write-Host ''
            Write-Host ("[环境不匹配] 与「本机正在跑的这套环境」对不上（{0}）" -f $envIssues.Count) -ForegroundColor Magenta
            foreach ($e in $envIssues) { Write-Host ("   - {0}`n       {1}  [{2}]" -f $e.Path, $e.Why, $e.Clause) -ForegroundColor Magenta }
        }
        if ($needOther.Count) {
            Write-Host ''
            Write-Host ("[需确认] 可能不该入库或需业主裁决（{0}）" -f $needOther.Count) -ForegroundColor Yellow
            foreach ($c in $needOther) { Write-Host ("   - {0}`n       {1}  [{2}]" -f $c.Path, $c.Why, $c.Clause) -ForegroundColor Yellow }
        }
        if ($notes.Count) {
            Write-Host ''
            Write-Host ("[提示] 已被跟踪的运行期产物（不阻断，建议按提示清理）（{0}）" -f $notes.Count) -ForegroundColor DarkGray
            foreach ($n in $notes) { Write-Host ("   - {0}`n       {1}  [{2}]" -f $n.Path, $n.Why, $n.Clause) -ForegroundColor DarkGray }
        }
        Write-Host ''
        if ($blocked.Count -or $needConfirm.Count) {
            Write-Host '结论：需要二次确认 —— 立即停止提交；把上面每条逐条报告业主（路径/类别/条款/影响/建议），' -ForegroundColor Red
            Write-Host '      取得业主【明确】确认后才可继续；例外条目写进提交信息正文。' -ForegroundColor Red
            Write-Host '      禁止 -Force / --no-verify / git add -A 绕过（根 AGENTS.md §0.3.1）。' -ForegroundColor Red
        }
        else {
            Write-Host '结论：通过（未发现缓存产物、规范违规或环境不匹配）' -ForegroundColor Green
        }
        Write-Host '提醒：本脚本只做机械判定；提交前仍须自行完成 go build ./... 、go vet ./... 、go test ./... -count=1，' -ForegroundColor DarkGray
        Write-Host '      并确认 git status --short 里只有本任务文件（§0.5）。' -ForegroundColor DarkGray
    }

    if ($blocked.Count -or $needConfirm.Count) { exit 2 }
    exit 0
}
finally {
    Pop-Location
}

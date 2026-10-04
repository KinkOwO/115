<#
.SYNOPSIS
  DFO 115us 提交前门禁：检查「本地缓存/构建产物入库」与「AGENTS §0.4/§0.5 规范违规」。

.DESCRIPTION
  只读脚本：只查询 git 与文件系统，不修改任何文件、不动暂存区。

  退出码：
    0 = 通过（可以直接提交）
    2 = 需要二次确认（发现缓存产物或规范违规）
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

    # ---- 4. 逐条判定 ----
    $blocked = @(); $needConfirm = @()
    foreach ($it in $items) {
        $p = $it.Path.Replace('\', '/')
        $isNew = ($it.Kind -eq 'A' -or $it.Kind -eq 'R')

        foreach ($rule in $denyPatterns) {
            if ($p -match $rule.Re) {
                $blocked += [pscustomobject]@{ Path = $it.Path; Why = $rule.Why; Clause = '§0.3.1 缓存/本地产物' }
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
    }

    # ---- 5. 输出 ----
    $scope = if ($All) { '工作树全部改动（含未跟踪）' } else { '已暂存内容' }
    if ($Json) {
        [pscustomobject]@{
            scope       = $scope
            paths       = $items.Count
            blocked     = $blocked
            needConfirm = $needConfirm
            verdict     = if ($blocked.Count -or $needConfirm.Count) { 'confirm-required' } else { 'pass' }
        } | ConvertTo-Json -Depth 5
    }
    else {
        Write-Host ''
        Write-Host 'DFO 115us · 提交前门禁（check-commit-hygiene）' -ForegroundColor Cyan
        Write-Host ("范围：{0}，共 {1} 个路径" -f $scope, $items.Count)
        if ($blocked.Count) {
            Write-Host ''
            Write-Host ("[违规] 缓存/本地产物或目录规范（{0}）" -f $blocked.Count) -ForegroundColor Red
            foreach ($b in $blocked) { Write-Host ("   - {0}`n       {1}  [{2}]" -f $b.Path, $b.Why, $b.Clause) -ForegroundColor Red }
        }
        if ($needConfirm.Count) {
            Write-Host ''
            Write-Host ("[需确认] 可能不该入库或需业主裁决（{0}）" -f $needConfirm.Count) -ForegroundColor Yellow
            foreach ($c in $needConfirm) { Write-Host ("   - {0}`n       {1}  [{2}]" -f $c.Path, $c.Why, $c.Clause) -ForegroundColor Yellow }
        }
        Write-Host ''
        if ($blocked.Count -or $needConfirm.Count) {
            Write-Host '结论：需要二次确认 —— 立即停止提交；把上面每条逐条报告业主（路径/类别/条款/影响/建议），' -ForegroundColor Red
            Write-Host '      取得业主【明确】确认后才可继续；例外条目写进提交信息正文。' -ForegroundColor Red
            Write-Host '      禁止 -Force / --no-verify / git add -A 绕过（根 AGENTS.md §0.3.1）。' -ForegroundColor Red
        }
        else {
            Write-Host '结论：通过（未发现缓存产物或规范违规）' -ForegroundColor Green
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

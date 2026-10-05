<#
.SYNOPSIS
  DFO 115us 存储路线切换器（SQLite 路线 / PostgreSQL 路线）。

.DESCRIPTION
  两条路线各有**自己的存档**，但全链（服务端、启动器、Web GM、GM 命令行）只读同一份活动档
  `server/work/dfo-lan/runtime/storage/local.json`，所以切换路线＝把选中的路线档写成活动档。
  本脚本只做三件事，不启动任何游戏程序：

    1. `use <路线>`：备份当前活动档为该路线的档（`local.<路线>.json`），再把目标路线档写成活动档；
       目标路线档不存在时按**本机路径**生成（不会去猜别的机器的路径）。
    2. `show`：报告当前路线、它连的是哪个库、另一条路线的档在不在。
    3. `stop-postgres`：停掉本仓库 `runtime\storage\pgdata` 上的 PostgreSQL（用路线档里的
       `postgres_bin`／`postgres_data`）。

  引擎判定与 `internal/database.EngineForConfig` 同一条规则（显式 driver > 有 DSN 选 PostgreSQL >
  只有 sqlite_path 选 SQLite），见 `server/work/dfo-lan/docs/sqlite-operations.md` §1.1。

  用法（也可用同目录的 `storage-route.cmd`）：
    powershell -NoProfile -ExecutionPolicy Bypass -File scripts\storage-route.ps1 show
    powershell -NoProfile -ExecutionPolicy Bypass -File scripts\storage-route.ps1 use sqlite
    powershell -NoProfile -ExecutionPolicy Bypass -File scripts\storage-route.ps1 use postgres
    powershell -NoProfile -ExecutionPolicy Bypass -File scripts\storage-route.ps1 stop-postgres

  退出码：0 = 成功；1 = 失败（调用方应当停止启动，不要带着半套配置去拉起服务端）。

.NOTES
  只写 `runtime\storage\` 下的配置档与活动档（该目录已被 .gitignore 忽略），不碰存档本体、
  不碰 PVF、不启动客户端。路线档缺 DSN 时给出项目的默认 DSN 并明确提示需要核对。
#>
param(
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$Rest
)

$ErrorActionPreference = 'Stop'

$RepoRoot = Split-Path -Parent $PSScriptRoot
$StorageDir = Join-Path $RepoRoot 'server\work\dfo-lan\runtime\storage'
$ActiveFile = Join-Path $StorageDir 'local.json'
$SqliteRouteFile = Join-Path $StorageDir 'local.sqlite.json'
$PostgresRouteFile = Join-Path $StorageDir 'local.postgres.json'
$SqliteFile = Join-Path $StorageDir 'dfolan.sqlite3'
$PgDataDir = Join-Path $StorageDir 'pgdata'
$LegacyPgBackup = Join-Path $StorageDir 'local.json.pg-backup'
# 便携 PostgreSQL 在仓库外（2026-10-04 tools 移出仓库）：仓库根上一级的 tools\pg。
$PgBinDir = Join-Path (Split-Path -Parent $RepoRoot) 'tools\pg\pgsql\bin'
# 与 scripts/configure_env.py 里的默认 DSN 一致；PG 路线首次生成时用它，并在输出里要求核对。
$DefaultDsn = 'postgres://dfo_owner:-J5vg5kBCfjt5WccbR1OkkXChKXxxqOBt1mZF6spNDI@127.0.0.1:25438/dfo_lan?sslmode=disable'

function Read-JsonFile([string]$path) {
    if (-not (Test-Path -LiteralPath $path)) { return $null }
    # BOM 容错：记事本 / Set-Content -Encoding UTF8 都会写 BOM，服务端与启动器都容忍它。
    $text = [System.IO.File]::ReadAllText($path)
    $text = $text.TrimStart([char]0xFEFF)
    if (-not $text.Trim()) { return $null }
    return ($text | ConvertFrom-Json)
}

function Write-JsonFile([string]$path, $table) {
    $dir = Split-Path -Parent $path
    if (-not (Test-Path -LiteralPath $dir)) { New-Item -ItemType Directory -Path $dir -Force | Out-Null }
    $json = ($table | ConvertTo-Json -Depth 5) + "`n"
    [System.IO.File]::WriteAllText($path, $json, (New-Object System.Text.UTF8Encoding($false)))
}

# JSON 对象 -> 有序哈希表：便于删键/改键，再按同一顺序写回。
function ConvertTo-OrderedTable($obj) {
    $table = [ordered]@{}
    if ($null -ne $obj) {
        foreach ($property in $obj.PSObject.Properties) { $table[$property.Name] = $property.Value }
    }
    return $table
}

# 引擎判定：与 internal/database.EngineForConfig 同一张表（见本文件 .DESCRIPTION）。
function Get-StorageDriver($table) {
    if ($table -and $table['driver'] -and ([string]$table['driver']).Trim()) {
        return ([string]$table['driver']).Trim().ToLower()
    }
    if ($table -and $table['postgres_dsn'] -and ([string]$table['postgres_dsn']).Trim()) { return 'postgres' }
    if ($table -and $table['sqlite_path'] -and ([string]$table['sqlite_path']).Trim()) { return 'sqlite' }
    return 'postgres'
}

# 仓库内的绝对路径写成正斜杠，和既有档例的风格一致。
function To-ConfigPath([string]$path) { return ($path -replace '\\', '/') }

function Test-Port([string]$hostName, [int]$port) {
    $client = New-Object System.Net.Sockets.TcpClient
    try {
        $task = $client.ConnectAsync($hostName, $port)
        if (-not $task.Wait(800)) { return $false }
        return $client.Connected
    }
    catch { return $false }
    finally { $client.Close() }
}

# 只在目录非空时才拼路径：Join-Path 传空串会直接报 "empty string" 而打断整个流程，
# 而「档里没写 postgres_data」是必须能如实报告的正常状态。
function Test-ChildFile([string]$dir, [string]$name) {
    if (-not $dir -or -not $dir.Trim()) { return $false }
    return (Test-Path -LiteralPath (Join-Path $dir $name))
}

function Get-DsnParts([string]$dsn) {
    $parts = [ordered]@{ host = '127.0.0.1'; port = 5432; database = 'dfo_lan' }
    if (-not $dsn) { return $parts }
    try {
        $uri = [System.Uri]$dsn
        if ($uri.Host) { $parts['host'] = $uri.Host }
        if ($uri.Port -gt 0) { $parts['port'] = $uri.Port }
        $name = $uri.AbsolutePath.TrimStart('/')
        if ($name) { $parts['database'] = $name }
    }
    catch { }
    return $parts
}

# SQLite 路线档：sqlite_path 必须是绝对路径（服务端明确拒绝相对路径）。
function New-SqliteRouteConfig() {
    $table = [ordered]@{}
    $table['driver'] = 'sqlite'
    $table['sqlite_path'] = (To-ConfigPath $SqliteFile)
    $table['sqlite_busy_timeout_ms'] = 5000
    $table['max_connections'] = 4
    return $table
}

# PostgreSQL 路线档：缺档时优先沿用本机既有的 PG 档（local.json.pg-backup），否则用默认 DSN，
# 并把 postgres_bin / postgres_data 修成本机实际存在的路径。
function New-PostgresRouteConfig([string]$seedPath) {
    $table = $null
    if ($seedPath -and (Test-Path -LiteralPath $seedPath)) {
        $table = ConvertTo-OrderedTable (Read-JsonFile $seedPath)
    }
    if ($null -eq $table) { $table = [ordered]@{} }
    $table['driver'] = 'postgres'
    if (-not $table['postgres_dsn'] -or -not ([string]$table['postgres_dsn']).Trim()) {
        $table['postgres_dsn'] = $DefaultDsn
        Write-Host "[提示] PG 路线档缺少 postgres_dsn，已写入项目默认 DSN；若与你本机 PostgreSQL 不同，请编辑 $PostgresRouteFile" -ForegroundColor Yellow
    }
    if (-not $table['max_connections']) { $table['max_connections'] = 12 }
    if (Test-Path -LiteralPath (Join-Path $PgBinDir 'pg_ctl.exe')) { $table['postgres_bin'] = $PgBinDir }
    if (Test-Path -LiteralPath (Join-Path $PgDataDir 'PG_VERSION')) { $table['postgres_data'] = $PgDataDir }
    # SQLite 专有键不清掉的话，这份 PG 档读起来就有两种可能（启动器与服务端都按 DSN 优先，
    # 但人读配置时最容易被带走）——与 configure_env.py 同一处理。
    foreach ($key in @('sqlite_path', 'sqlite_busy_timeout_ms', 'busy_timeout_ms', 'max_read_connections')) {
        if ($table.Contains($key)) { $table.Remove($key) }
    }
    return $table
}

function Write-RouteProfile([string]$route, $table) {
    if ($route -eq 'sqlite') { Write-JsonFile $SqliteRouteFile $table }
    else { Write-JsonFile $PostgresRouteFile $table }
}

function Show-Route([string]$route, $table) {
    $driver = Get-StorageDriver $table
    Write-Host ''
    Write-Host ("[路线] {0}（活动档 driver={1}）" -f $route, $driver) -ForegroundColor Cyan
    if ($driver -eq 'sqlite') {
        $path = [string]$table['sqlite_path']
        $exists = $path -and (Test-Path -LiteralPath $path)
        Write-Host ("  存档: SQLite  {0}  ({1})" -f $path, $(if ($exists) { '已存在' } else { '尚未创建，首次启动会建' }))
    }
    else {
        $parts = Get-DsnParts ([string]$table['postgres_dsn'])
        $listening = Test-Port $parts['host'] ([int]$parts['port'])
        Write-Host ("  存档: PostgreSQL {0}:{1}/{2}  ({3})" -f $parts['host'], $parts['port'], $parts['database'], $(if ($listening) { '监听中' } else { '未监听，启动器会拉起' }))
        $dataDir = ([string]$table['postgres_data']).Trim()
        Write-Host ("  数据目录: {0}  ({1})" -f $(if ($dataDir) { $dataDir } else { '(未配置)' }), $(if (Test-ChildFile $dataDir 'PG_VERSION') { '有效' } else { '无效或未配置' }))
        $binDir = ([string]$table['postgres_bin']).Trim()
        Write-Host ("  pg_ctl:   {0}  ({1})" -f $(if ($binDir) { $binDir } else { '(未配置)' }), $(if (Test-ChildFile $binDir 'pg_ctl.exe') { '存在' } else { '缺失' }))
    }
    Write-Host ("  另一条路线档: sqlite={0}  postgres={1}" -f (Test-Path -LiteralPath $SqliteRouteFile), (Test-Path -LiteralPath $PostgresRouteFile)) -ForegroundColor DarkGray
    Write-Host '  注意：两条路线的存档互相独立（SQLite 文件 ↔ PostgreSQL 库），切换路线不会带着角色走；' -ForegroundColor Yellow
    Write-Host '        需要搬运存档用 dfo-tool sqliteconvert（只支持 PostgreSQL → SQLite 单向）。' -ForegroundColor Yellow
}

function Use-Route([string]$route) {
    $route = $route.Trim().ToLower()
    if ($route -ne 'sqlite' -and $route -ne 'postgres') {
        throw "未知路线 '$route'；只认 sqlite / postgres。"
    }
    if (-not (Test-Path -LiteralPath $StorageDir)) { New-Item -ItemType Directory -Path $StorageDir -Force | Out-Null }

    $active = Read-JsonFile $ActiveFile
    if ($null -ne $active) {
        $activeTable = ConvertTo-OrderedTable $active
        $activeDriver = Get-StorageDriver $activeTable
        if ($activeDriver -eq $route) {
            # 同一条路线：把活动档原样刷成该路线的档，保住手工改过的内容。
            Write-RouteProfile $route $activeTable
        }
        elseif ($activeDriver -eq 'sqlite' -or $activeDriver -eq 'postgres') {
            # 换路线：先把当前活动档按它自己的引擎存下来，绝不丢配置。
            Write-RouteProfile $activeDriver $activeTable
            Write-Host ("[备份] 当前 {0} 档已存为 {1}" -f $activeDriver, $(if ($activeDriver -eq 'sqlite') { $SqliteRouteFile } else { $PostgresRouteFile })) -ForegroundColor DarkGray
        }
        else {
            Write-Host ("[提示] 当前活动档 driver={0} 不受支持，未备份它；按 {1} 路线继续。" -f $activeDriver, $route) -ForegroundColor Yellow
        }
    }

    $targetFile = if ($route -eq 'sqlite') { $SqliteRouteFile } else { $PostgresRouteFile }
    $target = $null
    if (Test-Path -LiteralPath $targetFile) { $target = ConvertTo-OrderedTable (Read-JsonFile $targetFile) }
    if ($null -eq $target -or -not $target.Count) {
        if ($route -eq 'sqlite') {
            $target = New-SqliteRouteConfig
            Write-Host '[生成] 已按本机路径生成 SQLite 路线档' -ForegroundColor DarkGray
        }
        else {
            $seed = $null
            if (Test-Path -LiteralPath $LegacyPgBackup) { $seed = $LegacyPgBackup }
            $target = New-PostgresRouteConfig $seed
            Write-Host ("[生成] 已生成 PostgreSQL 路线档（来源: {0}）" -f $(if ($seed) { $seed } else { '项目默认值' })) -ForegroundColor DarkGray
        }
    }
    # 路线档始终写回（把本次补的路径/默认值固化），再作为活动档。
    Write-RouteProfile $route $target
    Write-JsonFile $ActiveFile $target
    Show-Route $route $target

    if ($route -eq 'sqlite') {
        if (Test-Port '127.0.0.1' 25438) {
            Write-Host '[提示] PostgreSQL 仍在监听 25438（上一条 PG 路线遗留）；要停它就运行：scripts\storage-route.cmd stop-postgres' -ForegroundColor Yellow
        }
    }
}

# SQLite 管理租约（GM 互斥）自愈。
#
# 症状（2026-10-05 实机）：服务端启动被拒 ——
#   「已有 GM 写入正在进行（SQLite 管理租约文件 …admin-guard 由进程 13248 持有，最近一次续租 24s 前）」
# 原因是上一次会话被强杀（停止脚本 taskkill / Ctrl+C），租约要等 60 秒 TTL 才失效，期间起不来。
# 本动作只在**确认记录里的进程已不存在**后删除租约；记录里的进程还活着（多半是 Web GM 在写）
# 就拒绝并打印它是谁——绝不代替业主抢锁（那正是这个锁要防的事）。
function Clear-AdminGuard() {
    $active = Read-JsonFile $ActiveFile
    if ($null -eq $active) { throw "活动档不存在：$ActiveFile" }
    $table = ConvertTo-OrderedTable $active
    $driver = Get-StorageDriver $table
    if ($driver -ne 'sqlite') {
        Write-Host ("[跳过] 当前是 {0} 路线；管理租约只用于 SQLite 档（PostgreSQL 用 advisory lock，连接断开即释放）。" -f $driver) -ForegroundColor DarkGray
        return
    }
    $dbPath = ([string]$table['sqlite_path']).Trim()
    if (-not $dbPath) { throw '当前 SQLite 档缺少 sqlite_path，无法定位管理租约。' }
    $lease = "$dbPath.admin-guard"
    if (-not (Test-Path -LiteralPath $lease)) {
        Write-Host '[跳过] 没有管理租约文件，服务端可以直接启动。' -ForegroundColor Green
        return
    }
    $holder = (Get-Content -LiteralPath $lease -Raw -ErrorAction SilentlyContinue)
    $pidText = if ($holder) { $holder.Trim() } else { '' }
    $age = (Get-Date) - (Get-Item -LiteralPath $lease).LastWriteTime
    Write-Host ("[租约] {0}`n        持有者 pid={1}；最近续租 {2:N0} 秒前" -f $lease, $(if ($pidText) { $pidText } else { '(空)' }), $age.TotalSeconds)
    if ($pidText -notmatch '^\d+$') {
        # 读不出 pid（空文件/内容异常）不删：那也可能是持有者刚创建、还没写 pid 的瞬间，
        # 删了会让两个写者同时进。等 TTL，或由业主确认后手工删除。
        throw ("租约里没有可用的 pid（内容 '{0}'）：不自动删除。等 60 秒 TTL 后再试，或确认无人写入后手工删除 {1}" -f $pidText, $lease)
    }
    $live = Get-Process -Id ([int]$pidText) -ErrorAction SilentlyContinue
    if ($live) {
        throw ("持有者 pid {0}（{1}）仍在运行，可能正在写存档：拒绝删除租约。确认它已退出（或等 60 秒 TTL）后再试。" -f $pidText, $live.ProcessName)
    }
    Remove-Item -LiteralPath $lease -Force
    Write-Host '[完成] 已删除过期管理租约（记录的进程已不存在），现在可以启动服务端了。' -ForegroundColor Green
}

function Stop-Postgres() {
    $source = $null
    foreach ($candidate in @($PostgresRouteFile, $ActiveFile, $LegacyPgBackup)) {
        if (-not (Test-Path -LiteralPath $candidate)) { continue }
        $table = ConvertTo-OrderedTable (Read-JsonFile $candidate)
        if ((Get-StorageDriver $table) -eq 'postgres') { $source = $table; break }
    }
    if ($null -eq $source) { throw '找不到任何 PostgreSQL 路线档，无法确定要停哪个实例。' }
    $bin = [string]$source['postgres_bin']
    $data = [string]$source['postgres_data']
    $parts = Get-DsnParts ([string]$source['postgres_dsn'])
    $pgCtl = Join-Path $bin 'pg_ctl.exe'
    if (-not (Test-Path -LiteralPath $pgCtl)) { throw "找不到 pg_ctl：$pgCtl（先跑 配置环境.cmd 或编辑 PG 路线档）" }
    if (-not (Test-Path -LiteralPath (Join-Path $data 'PG_VERSION'))) { throw "数据目录无效：$data" }
    if (-not (Test-Port $parts['host'] ([int]$parts['port']))) {
        Write-Host ("[跳过] PostgreSQL {0}:{1} 未在监听，无需停止。" -f $parts['host'], $parts['port']) -ForegroundColor DarkGray
        return
    }
    Write-Host "[停止] pg_ctl -D $data -m fast -w -t 20 stop"
    & $pgCtl -D $data -m fast -w -t 20 stop 2>&1 | ForEach-Object { Write-Host "  $_" }
    if (Test-Port $parts['host'] ([int]$parts['port'])) { throw 'PostgreSQL 仍在监听，停止未成功。' }
    Write-Host '[完成] PostgreSQL 已停止（数据目录原样保留）。' -ForegroundColor Green
}

function Show-Current() {
    $active = Read-JsonFile $ActiveFile
    if ($null -eq $active) {
        Write-Host "[路线] 活动档不存在：$ActiveFile" -ForegroundColor Yellow
        Write-Host '  用 scripts\storage-route.cmd use sqlite 或 use postgres 建立第一条路线。'
        return
    }
    $table = ConvertTo-OrderedTable $active
    $driver = Get-StorageDriver $table
    if ($driver -eq 'sqlite' -or $driver -eq 'postgres') { Show-Route $driver $table }
    else { Write-Host "[路线] 活动档 driver=$driver 不受支持（只认 sqlite / postgres）；文件：$ActiveFile" -ForegroundColor Red }
}

# 自检：把脚本作用域的路径整体指向临时目录，验证引擎判定表与两条路线的往返，
# 全程不碰本机真实配置，也不在仓库里留下产物。
function Invoke-SelfTest() {
    $tempRoot = Join-Path $env:TEMP ('dfo-storage-selftest-' + [guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $tempRoot -Force | Out-Null
    $saved = @{
        ActiveFile        = $script:ActiveFile
        SqliteRouteFile   = $script:SqliteRouteFile
        PostgresRouteFile = $script:PostgresRouteFile
        SqliteFile        = $script:SqliteFile
        PgDataDir         = $script:PgDataDir
        LegacyPgBackup    = $script:LegacyPgBackup
        PgBinDir          = $script:PgBinDir
    }
    $checks = 0
    try {
        $script:ActiveFile = Join-Path $tempRoot 'local.json'
        $script:SqliteRouteFile = Join-Path $tempRoot 'local.sqlite.json'
        $script:PostgresRouteFile = Join-Path $tempRoot 'local.postgres.json'
        $script:SqliteFile = Join-Path $tempRoot 'dfolan.sqlite3'
        $script:PgDataDir = Join-Path $tempRoot 'pgdata'
        $script:LegacyPgBackup = Join-Path $tempRoot 'local.json.pg-backup'
        $script:PgBinDir = Join-Path $tempRoot 'pgbin'

        # 1. 引擎判定表：与 internal/database/engine_selection_test.go 同一张表。
        $table = @(
            @{ cfg = [ordered]@{ driver = 'sqlite' }; want = 'sqlite' },
            @{ cfg = [ordered]@{ driver = 'postgres' }; want = 'postgres' },
            @{ cfg = [ordered]@{ driver = ' SQLite ' }; want = 'sqlite' },
            @{ cfg = [ordered]@{ postgres_dsn = 'postgres://u@127.0.0.1:25438/dfo_lan'; sqlite_path = 'leftover.sqlite3' }; want = 'postgres' },
            @{ cfg = [ordered]@{ postgres_dsn = 'postgres://u@127.0.0.1:25438/dfo_lan' }; want = 'postgres' },
            @{ cfg = [ordered]@{ sqlite_path = 'dfolan.sqlite3' }; want = 'sqlite' },
            @{ cfg = [ordered]@{}; want = 'postgres' },
            @{ cfg = [ordered]@{ postgres_dsn = '  '; sqlite_path = 'dfolan.sqlite3' }; want = 'sqlite' }
        )
        foreach ($case in $table) {
            $got = Get-StorageDriver $case.cfg
            if ($got -ne $case.want) {
                throw ("引擎判定不符：{0} 得到 {1}，期望 {2}" -f ($case.cfg | ConvertTo-Json -Compress), $got, $case.want)
            }
            $checks++
        }

        # 2. SQLite 路线：必须写出绝对 sqlite_path（服务端拒绝相对路径）。
        Use-Route 'sqlite' 6>$null
        $active = ConvertTo-OrderedTable (Read-JsonFile $script:ActiveFile)
        if ((Get-StorageDriver $active) -ne 'sqlite') { throw 'use sqlite 之后活动档不是 sqlite' }
        if (-not [System.IO.Path]::IsPathRooted([string]$active['sqlite_path'])) { throw "sqlite_path 不是绝对路径：$($active['sqlite_path'])" }
        $checks++

        # 3. 切到 PostgreSQL：SQLite 档必须被原样保留（换路线不能丢配置）。
        Use-Route 'postgres' 6>$null
        $active = ConvertTo-OrderedTable (Read-JsonFile $script:ActiveFile)
        if ((Get-StorageDriver $active) -ne 'postgres') { throw 'use postgres 之后活动档不是 postgres' }
        if (-not (Test-Path -LiteralPath $script:SqliteRouteFile)) { throw '切到 postgres 后没有留下 local.sqlite.json' }
        $kept = ConvertTo-OrderedTable (Read-JsonFile $script:SqliteRouteFile)
        if ((Get-StorageDriver $kept) -ne 'sqlite') { throw 'local.sqlite.json 里的 driver 不是 sqlite' }
        $checks++

        # 4. 切回 SQLite：PostgreSQL 档同样必须保留。
        Use-Route 'sqlite' 6>$null
        if (-not (Test-Path -LiteralPath $script:PostgresRouteFile)) { throw '切回 sqlite 后没有留下 local.postgres.json' }
        $active = ConvertTo-OrderedTable (Read-JsonFile $script:ActiveFile)
        if ((Get-StorageDriver $active) -ne 'sqlite') { throw '切回 sqlite 之后活动档不是 sqlite' }
        $checks++

        # 5. 未知路线必须失败，而不是默默按 PostgreSQL 处理。
        $rejected = $false
        try { Use-Route 'mysql' 6>$null } catch { $rejected = $true }
        if (-not $rejected) { throw '未知路线没有被拒绝' }
        $checks++

        # 6. 「两个路线脚本各自独立可用」＝ Go 启动器必须在位（业主定调：只走 Go，
        #    无 Python、无外部启动器、无回退），而且两个游戏入口文件在位。
        foreach ($entry in @('启动游戏-SQLite.cmd', '启动游戏-PostgreSQL.cmd')) {
            $target = Join-Path $PSScriptRoot $entry
            if (-not (Test-Path -LiteralPath $target)) { throw "路线入口不存在：$target" }
            $checks++
        }
        foreach ($scope in @('', '--server-only')) {
            # @() 包裹是必须的：单元素数组会被 PowerShell 解包成对象，.Count 就成了 $null
            # （2026-10-05，与门禁脚本同一个坑）。
            $scoped = @(Get-FullChainCandidates $scope)
            if ($scoped.Count -eq 0) { throw ('缺少仓库内 Go 启动器：{0}' -f (Get-GoLauncherPath)) }
            if ($scoped.Count -ne 1 -or $scoped[0].Kind -ne 'go-launcher') { throw '启动链里出现了非 Go 的候选（违反「只走 Go」）' }
            if ($scope -and ($scoped[0].Args -notcontains '--server-only')) { throw 'server-only 变体没有把 --server-only 传给启动器' }
            $checks++
        }
        if ($GoLauncherEnvRequireIsolation -ne 'DFO_REQUIRE_GO_ISOLATION') { throw '强制 Go 隔离的环境变量名写错了' }
        $checks++

        # 7. 管理租约自愈：记录的 pid 不存在 ⇒ 删掉；记录的 pid 就是本进程 ⇒ 必须拒绝。
        $lease = "$($script:SqliteFile).admin-guard"
        [System.IO.File]::WriteAllText($lease, '999999', (New-Object System.Text.UTF8Encoding($false)))
        Clear-AdminGuard 6>$null
        if (Test-Path -LiteralPath $lease) { throw '过期管理租约没有被清理' }
        $checks++
        [System.IO.File]::WriteAllText($lease, "$PID", (New-Object System.Text.UTF8Encoding($false)))
        $refused = $false
        try { Clear-AdminGuard 6>$null } catch { $refused = $true }
        if (-not $refused) { throw '持有者仍存活时没有拒绝删除租约' }
        if (-not (Test-Path -LiteralPath $lease)) { throw '拒绝之后租约文件不该消失' }
        Remove-Item -LiteralPath $lease -Force
        $checks++

        Write-Host ("[自检] OK：{0} 项检查通过（临时目录 {1}；本机真实配置未改动）" -f $checks, $tempRoot) -ForegroundColor Green
        return $true
    }
    catch {
        Write-Host ("[自检] 失败：" + $_.Exception.Message) -ForegroundColor Red
        return $false
    }
    finally {
        $script:ActiveFile = $saved.ActiveFile
        $script:SqliteRouteFile = $saved.SqliteRouteFile
        $script:PostgresRouteFile = $saved.PostgresRouteFile
        $script:SqliteFile = $saved.SqliteFile
        $script:PgDataDir = $saved.PgDataDir
        $script:LegacyPgBackup = $saved.LegacyPgBackup
        $script:PgBinDir = $saved.PgBinDir
        Remove-Item -LiteralPath $tempRoot -Recurse -Force -ErrorAction SilentlyContinue
    }
}

# PostgreSQL 起库预检：有上限、有进度，绝不静默等待。
#
# 现场（2026-10-05 11:56）：业主双击 PostgreSQL 路线入口，PG 日志已到
# `database system is ready to accept connections`，但控制台停在「检查并拉起 PostgreSQL…」
# 没有任何后续输出，业主等了一分钟按 Ctrl+C（PG 日志留下 `received fast shutdown request` +
# `^C`，0xC000013A），数据目录里留下一个 pid 已死的 postmaster.pid。
#
# 所以这里做三件事：
#   1. 端口已经开着 ⇒ 直接跳过起库（最常见的情形，秒过）；
#   2. 数据目录里有**残留锁文件**（postmaster.pid 的 pid 已不存在）⇒ 先清掉再说，
#      并打印它清掉了什么（PG 自己也会清，但先清掉能让起库不再卡在那一步）；
#   3. 起库调用**有上限**（默认 60 秒）并且每秒打印一次进度；超时就终止它、打印该怎么查，
#      然后继续交给统一入口——启动器自己还会再试一次，不让入口卡住。
function Get-PostmasterPid([string]$dataDir) {
    $pidFile = Join-Path $dataDir 'postmaster.pid'
    if (-not (Test-Path -LiteralPath $pidFile)) { return 0 }
    $first = Get-Content -LiteralPath $pidFile -TotalCount 1 -ErrorAction SilentlyContinue
    if ($first -and $first.Trim() -match '^\d+$') { return [int]$first.Trim() }
    return 0
}

function Clear-StalePostmasterLock([string]$dataDir) {
    $holder = Get-PostmasterPid $dataDir
    if ($holder -le 0) { return }
    if (Get-Process -Id $holder -ErrorAction SilentlyContinue) {
        Write-Host ("[存储] 数据目录的 postmaster.pid 记录着 pid {0}（进程仍在）——不动它。" -f $holder) -ForegroundColor DarkGray
        return
    }
    $pidFile = Join-Path $dataDir 'postmaster.pid'
    try {
        Remove-Item -LiteralPath $pidFile -Force
        Write-Host ("[存储] 清掉残留锁文件 postmaster.pid（记录的 pid {0} 已不存在）：{1}" -f $holder, $pidFile) -ForegroundColor Yellow
    }
    catch {
        Write-Host ("[存储] 残留锁文件删不掉（{0}）：继续起库，PG 会自己处理。" -f $_.Exception.Message) -ForegroundColor Yellow
    }
}

function Start-PostgresBounded([int]$timeoutSeconds) {
    if (Test-Port '127.0.0.1' 25438) {
        Write-Host '[存储] PostgreSQL 已在监听 25438，跳过起库。' -ForegroundColor Green
        return
    }
    $cfg = ConvertTo-OrderedTable (Read-JsonFile $PostgresRouteFile)
    $dataDir = ([string]$cfg['postgres_data']).Trim()
    $binDir = ([string]$cfg['postgres_bin']).Trim()
    if ($dataDir) { Clear-StalePostmasterLock $dataDir }
    if (-not $binDir) {
        Write-Host '[存储] PG 路线档缺少 postgres_bin，跳过预检起库（交给统一入口处理）。' -ForegroundColor Yellow
        return
    }
    $pgCtl = Join-Path $binDir 'pg_ctl.exe'
    if (-not (Test-Path -LiteralPath $pgCtl) -or -not (Test-Path -LiteralPath (Join-Path $dataDir 'PG_VERSION'))) {
        Write-Host ("[存储] pg_ctl 或数据目录不可用（{0} / {1}），跳过预检起库。" -f $pgCtl, $dataDir) -ForegroundColor Yellow
        return
    }

    # 直接用 pg_ctl 起库，但**必须让它彻底脱离我们的句柄**，而且不等待它。
    #
    # 为什么（2026-10-05 实机「PG 日志已 ready、控制台却卡在拉起 PostgreSQL」，本机逐步复现）：
    # Windows 上 `pg_ctl start` 会留一个 cmd.exe 包装器当 postgres 的父进程，它继承调用者的
    # stdout/stderr；于是任何「把子进程输出接成管道再等 EOF」或「共享控制台再等它」的写法都会
    # 一直等下去——PG 早就 ready，调用方永远不返回。实测这三种写法都卡：
    #   * Go 的 cmd.CombinedOutput()（internal/launcher/storage.go，已改成写文件的 Run）；
    #   * PowerShell `& pg_ctl ...`（同步等待 + 共享控制台）；
    #   * PowerShell 5.1 的 Start-Process -RedirectStandard*（退出时要收尾那些重定向流）。
    # 可靠写法：**用 WMI 创建进程**（Win32_Process.Create 不继承调用者句柄），命令行内部自己
    # 重定向 `< NUL >> ctl.log 2>&1`（包装器继承到的是文件），然后**只轮询端口**判断就绪。
    $logPath = Join-Path $StorageDir 'postgres.log'
    $ctlOut = Join-Path $StorageDir 'pg-ctl.out.log'
    $wait = [Math]::Min([Math]::Max($timeoutSeconds, 10), 60)
    Write-Host ("[存储] 起库：pg_ctl start -w -t {0}（PG 日志 {1}；pg_ctl 输出 {2}）…" -f $wait, $logPath, $ctlOut) -ForegroundColor Cyan
    $line = 'cmd /c ""{0}" start -D "{1}" -l "{2}" -w -t {3} < NUL >> "{4}" 2>&1"' -f $pgCtl, $dataDir, $logPath, $wait, $ctlOut
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    $created = $null
    try {
        $created = Invoke-CimMethod -ClassName Win32_Process -MethodName Create -Arguments @{ CommandLine = $line } -ErrorAction Stop
    }
    catch {
        Write-Host ("[存储] 经 WMI 拉起 pg_ctl 失败（{0}），交给统一入口处理。" -f $_.Exception.Message) -ForegroundColor Yellow
        return
    }
    if ($null -eq $created -or $created.ReturnValue -ne 0) {
        Write-Host ("[存储] 经 WMI 拉起 pg_ctl 失败（ReturnValue={0}），交给统一入口处理。" -f $created.ReturnValue) -ForegroundColor Yellow
        return
    }

    $waited = 0
    while ($waited -lt $wait) {
        if (Test-Port '127.0.0.1' 25438) { break }
        Start-Sleep -Seconds 1
        $waited++
        if ($waited % 5 -eq 0) { Write-Host ("      … 起库中：{0}/{1} 秒（{2}）" -f $waited, $wait, $logPath) -ForegroundColor DarkGray }
    }
    $elapsed = [math]::Round($sw.Elapsed.TotalSeconds, 1)

    if (Test-Port '127.0.0.1' 25438) {
        Write-Host ("[存储] PostgreSQL 已就绪（{0} 秒）。" -f $elapsed) -ForegroundColor Green
        return
    }
    Write-Host ("[存储] 等了 {0} 秒 25438 仍未监听；交给统一入口处理。" -f $elapsed) -ForegroundColor Yellow
    Write-Host ("        排查：{0}、数据目录里的 postmaster.pid，或先跑 scripts\停止游戏环境.cmd。" -f $logPath) -ForegroundColor Yellow
}

# ── 完整启动链：只有仓库内的 Go 启动器，没有任何外部/Python 回退 ──────────────
#
# 业主定调（2026-10-05，第三次明确）：**彻底移除所有外部环境依赖，包括 Python；
# 不要再回退，必须 Go 成功**。所以这里只认 `server\work\dfo-lan\bin\dfolauncher.exe`
# （由 `go build ./cmd/dfolauncher` 从仓库源码产出，运行期不需要 Python / 不需要外部启动器）：
#   * 存储（SQLite 文件 / PostgreSQL 实例）：Go 的 storage 路径
#   * 内层 PVF：Go 的 innerpvf 路径
#   * 网关（含 session fixture、run.json、就绪轮询）：Go 的 serverrun/gateway/fixture 路径
#   * 客户端：Go 宿主 + Go WFP 隔离（clienthost），并强制 `DFO_REQUIRE_GO_ISOLATION=1`
#     —— 隔离装不上就**报错停下**，不再静默回退 probe.exe。
# 缺二进制时直接失败并说明怎么构建，绝不改用别的解释器或别的启动器。
$GoLauncherEnvRequireIsolation = 'DFO_REQUIRE_GO_ISOLATION'

function Get-GoLauncherPath() {
    return (Join-Path $RepoRoot 'server\work\dfo-lan\bin\dfolauncher.exe')
}

function Get-FullChainCandidates([string]$scope) {
    $scopeArgs = if ($scope) { @($scope) } else { @() }
    $candidates = @()
    $goLauncher = Get-GoLauncherPath
    if (Test-Path -LiteralPath $goLauncher) {
        $candidates += [pscustomobject]@{ Kind = 'go-launcher'; Path = $goLauncher; Args = @('launch') + $scopeArgs }
    }
    return $candidates
}

function Show-ChainInfo() {
    $candidates = Get-FullChainCandidates ''
    $goLauncher = Get-GoLauncherPath
    Write-Host ''
    Write-Host '[完整启动链] 只走仓库内的 Go 启动器（无 Python、无外部启动器、无回退）：' -ForegroundColor Cyan
    if (-not $candidates) {
        Write-Host ("  缺少 {0}" -f $goLauncher) -ForegroundColor Red
        Write-Host '  构建（需要 Go 工具链，只在开发机上做一次）：' -ForegroundColor Yellow
        Write-Host '    cd server\work\dfo-lan' -ForegroundColor Yellow
        Write-Host '    go build -trimpath -o bin\dfolauncher.exe .\cmd\dfolauncher' -ForegroundColor Yellow
        return
    }
    Write-Host ("  {0} launch [--server-only]" -f $candidates[0].Path)
    Write-Host ("  强制 Go：{0}=1（Go 隔离不可用时直接报错，不回退 probe.exe）" -f $GoLauncherEnvRequireIsolation) -ForegroundColor DarkGray
}

function Invoke-FullChain([string]$route, [string]$scope, [string[]]$extra) {
    Use-Route $route
    if ($route -eq 'postgres') {
        Start-PostgresBounded 60
    }
    $candidates = Get-FullChainCandidates $scope
    if (-not $candidates) {
        throw ('缺少仓库内的 Go 启动器 {0}。构建一次（需要 Go 工具链）：cd server\work\dfo-lan; go build -trimpath -o bin\dfolauncher.exe .\cmd\dfolauncher' -f (Get-GoLauncherPath))
    }
    $chosen = $candidates[0]
    $all = @($chosen.Args) + @($extra)
    Write-Host ''
    Write-Host ("[启动] 仓库内 Go 启动器（{0} 路线{1}）" -f $route, $(if ($scope) { '，只起服务端' } else { '' })) -ForegroundColor Cyan
    Write-Host ("       {0} {1}" -f $chosen.Path, ($all -join ' '))
    # 「不要再回退了，必须 Go 成功」：强制 Go 客户端宿主 + Go WFP 隔离；
    # 隔离装不上时 clientrun.go 会**报错停下**，不会静默改用 probe.exe。
    $env:DFO_REQUIRE_GO_ISOLATION = '1'
    Write-Host ("       环境：{0}=1（Go 隔离不可用就报错，不回退）" -f $GoLauncherEnvRequireIsolation) -ForegroundColor DarkGray
    Set-Location $RepoRoot
    & $chosen.Path @all
    exit $LASTEXITCODE
}

function Show-Help() {
    Write-Host ''
    Write-Host 'DFO 115us 存储路线（双库双路线）' -ForegroundColor Cyan
    Write-Host '  两条路线各有自己的存档，活动档 = server\work\dfo-lan\runtime\storage\local.json'
    Write-Host '  启动链只有仓库内的 Go 启动器：无 Python、无外部启动器、无回退。'
    Write-Host ''
    Write-Host '  启动入口（双击即可，也可带参数，如 --source-build）：'
    Write-Host '    scripts\启动游戏-SQLite.cmd        游戏全链，SQLite 存档（不需要 PostgreSQL）'
    Write-Host '    scripts\启动游戏-PostgreSQL.cmd    游戏全链，PostgreSQL 存档（自动起库）'
    Write-Host '    scripts\启动服务端-SQLite.cmd      只起服务端，SQLite'
    Write-Host '    scripts\启动服务端-PostgreSQL.cmd  只起服务端，PostgreSQL'
    Write-Host ''
    Write-Host '  切换与检查：'
    Write-Host '    scripts\storage-route.cmd show              当前路线 / 连的是哪个库'
    Write-Host '    scripts\storage-route.cmd use sqlite|postgres'
    Write-Host '    scripts\storage-route.cmd chain-info        只看会调哪个启动器、带哪些强制开关'
    Write-Host '    scripts\storage-route.cmd stop-postgres     停掉 pgdata 上的 PostgreSQL'
    Write-Host '    scripts\storage-route.cmd preflight-postgres 只做起库预检（清残留锁 + 有上限地拉起 PG）'
    Write-Host '    scripts\storage-route.cmd clear-guard        清理过期 SQLite 管理租约（上次会话被强杀后起不来时用）'
    Write-Host '    scripts\storage-route.cmd selftest          自检（临时目录，不碰真实配置）'
    Write-Host ''
    Write-Host '  说明见 server\work\dfo-lan\docs\sqlite-operations.md §1.2。' -ForegroundColor DarkGray
}

try {
    $verb = if ($Rest.Count -ge 1) { $Rest[0].Trim().ToLower() } else { 'help' }
    $extra = if ($Rest.Count -ge 2) { @($Rest[1..($Rest.Count - 1)]) } else { @() }
    switch ($verb) {
        'show' { Show-Current }
        'use' {
            if ($Rest.Count -lt 2) { throw 'use 需要路线：use sqlite | use postgres' }
            Use-Route $Rest[1]
        }
        'stop-postgres' { Stop-Postgres }
        'preflight-postgres' { Start-PostgresBounded 60 }
        'clear-guard' { Clear-AdminGuard }
        'chain-info' { Show-ChainInfo }
        'selftest' {
            if (-not (Invoke-SelfTest)) { exit 1 }
        }
        'game-sqlite' { Invoke-FullChain 'sqlite' '' $extra }
        'game-postgres' { Invoke-FullChain 'postgres' '' $extra }
        # 保持当前活动档不变，只把全链拉起来：scripts\启动游戏.cmd 用这个——
        # 它不该私自换路线，路线只由两个路线入口决定。
        'game-current' {
            $active = ConvertTo-OrderedTable (Read-JsonFile $ActiveFile)
            Invoke-FullChain (Get-StorageDriver $active) '' $extra
        }
        'server-sqlite' { Invoke-FullChain 'sqlite' '--server-only' $extra }
        'server-postgres' { Invoke-FullChain 'postgres' '--server-only' $extra }
        'help' { Show-Help }
        default { Show-Help; throw "未知动作 '$verb'。" }
    }
    exit 0
}
catch {
    Write-Host ("[失败] " + $_.Exception.Message) -ForegroundColor Red
    # 退出码 3 = 本脚本自己的失败（切换路线 / 起库 / 找不到完整启动链）。四个入口据此 pause，
    # 让业主看得到原因；成功路径由 Invoke-FullChain 直接 exit 子进程的退出码，不走这里。
    exit 3
}

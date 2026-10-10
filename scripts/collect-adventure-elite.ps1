<#
.SYNOPSIS
  核对精锐候选或汇总一次手动会话；不启动客户端、不修改存档/配置。
.NOTES
  -CheckOnly 为只读检查，不创建目录或文件。普通模式仅复制日志、写诊断汇总到 runtime/。
  退出码：0 检查/采集完成（不代表玩法验收）；1 文件/格式错误；2 候选缺失或哈希不匹配。
  PatchDirectory/OutputRoot/SessionPath 供离线夹具和指定会话使用；写入只能落在本工作树。
#>
[CmdletBinding()]
param(
    [switch]$CheckOnly,
    [string]$SessionPath,
    [string]$PatchDirectory,
    [string]$OutputRoot
)
$ErrorActionPreference = 'Stop'
$taskRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..')).Path
Set-Location -LiteralPath $taskRoot
$utf8 = New-Object Text.UTF8Encoding($false)
if (!$PatchDirectory) { $PatchDirectory = Join-Path $taskRoot 'client-patchs/adventure-elite/dist' }
if (!$OutputRoot) { $OutputRoot = Join-Path $taskRoot 'server/work/dfo-lan/runtime/adventure-elite-captures' }
function Read-SharedText([string]$path) {
    $stream = New-Object IO.FileStream($path, [IO.FileMode]::Open, [IO.FileAccess]::Read, [IO.FileShare]::ReadWrite)
    $reader = New-Object IO.StreamReader($stream, [Text.Encoding]::UTF8, $true)
    try { return $reader.ReadToEnd() } finally { $reader.Dispose() }
}
function Read-Json([string]$path) {
    if (!(Test-Path -LiteralPath $path -PathType Leaf)) { return $null }
    return ([IO.File]::ReadAllText($path).TrimStart([char]0xfeff) | ConvertFrom-Json)
}
function Read-Lines([string]$path, [System.Collections.Generic.List[string]]$errors) {
    $rows = New-Object 'System.Collections.Generic.List[object]'
    if (!(Test-Path -LiteralPath $path -PathType Leaf)) { return ,$rows }
    $reader = [IO.File]::OpenText($path)
    $lineNumber = 0
    try {
        while ($null -ne ($line = $reader.ReadLine())) {
            $lineNumber++
            if (!$line.Trim()) { continue }
            try { $rows.Add(($line | ConvertFrom-Json)) }
            catch { $errors.Add(('无法解析 {0} 第 {1} 行' -f (Split-Path $path -Leaf), $lineNumber)) }
        }
    } finally { $reader.Dispose() }
    return ,$rows
}
$supportedVersions = @('0.3.1', '0.3.2', '0.3.3', '0.3.4','0.3.5','0.3.6','0.3.7','0.3.8','0.3.9','0.3.10')
$build = Read-Json (Join-Path $PatchDirectory 'adventure-elite-build.json')
$artifactChecks = @()
$candidateValid = $null -ne $build -and $build.version -in $supportedVersions
if ($null -ne $build) {
    foreach ($artifact in $build.artifacts) {
        # Metadata paths are data: reject traversal before reading.
        if ($artifact.path -notin @('client-patchs/adventure-elite/dist/AdventureElite.dll', 'server/work/dfo-lan/bin/wireprobe-handoff-source.exe', 'server/work/dfo-lan/bin/dfolauncher-adventure-elite.exe')) {
            throw '候选清单出现不支持的路径。'
        }
        $path = if ($artifact.path.EndsWith('AdventureElite.dll')) { Join-Path $PatchDirectory 'AdventureElite.dll' } else { Join-Path $taskRoot $artifact.path }
        $exists = Test-Path -LiteralPath $path -PathType Leaf
        $actual = if ($exists) { (Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash.ToLowerInvariant() } else { '' }
        $match = $exists -and $actual -eq $artifact.sha256
        $candidateValid = $candidateValid -and $match
        $artifactChecks += [pscustomobject]@{ path = $artifact.path; exists = $exists; match = $match; sha256 = $actual }
    }
    $candidateValid = $candidateValid -and @($artifactChecks).Count -eq 3
}
$status = Read-Json (Join-Path $PatchDirectory 'adventure-elite.status.json')
$resourceChecks = @()
foreach ($resource in $build.resources) {
    if ($resource.path -notin @('client/DFO.exe', 'client/Script.pvf', 'client/sk.dat', 'server/work/client-build/Script.inner.pvf')) { throw '资源身份清单出现不支持的路径。' }
    $path = Join-Path $taskRoot $resource.path
    $exists = Test-Path -LiteralPath $path -PathType Leaf
    $actual = if ($exists) { (Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash.ToLowerInvariant() } else { '' }
    $resourceChecks += [pscustomobject]@{ path = $resource.path; exists = $exists; sha256 = $actual; sameAsBuildInput = ($exists -and $actual -eq $resource.sha256) }
}
if ($CheckOnly) {
    [pscustomobject]@{ candidateValid = $candidateValid; expectedVersion = $build.version; artifacts = $artifactChecks; resources = $resourceChecks; lastRunVersion = $status.version; lastRunPid = $status.processId; lastRunDllVersionMatches = ($status.version -eq $build.version); lastRunServerBuildVerified = $false } | ConvertTo-Json -Depth 6
    if (!$candidateValid) { exit 2 }
    Write-Host '只读预检通过；未启动游戏，旧状态不代表新候选已运行。'
    exit 0
}
if (!$SessionPath) {
    $sessions = @(Get-ChildItem -LiteralPath (Join-Path $taskRoot 'server/work/dfo-lan/runtime') -Directory -Filter 'roles_*' | Sort-Object LastWriteTime -Descending | Select-Object -First 50)
    foreach ($session in $sessions) {
        $clientLog = Join-Path $session.FullName 'client.log'
        if ($status.processId -and (Test-Path -LiteralPath $clientLog) -and (Read-SharedText $clientLog) -match ('(?m)ROOT_PID\s+' + [regex]::Escape([string]$status.processId) + '\s*$')) {
            $SessionPath = $session.FullName
            break
        }
    }
    if (!$SessionPath -and $sessions.Count) { $SessionPath = $sessions[0].FullName }
}
if (!$SessionPath) { throw '未找到手动游戏会话；不启动游戏。' }
$SessionPath = (Resolve-Path -LiteralPath $SessionPath).Path
$outputAbsolute = [IO.Path]::GetFullPath($OutputRoot)
if (!$outputAbsolute.StartsWith($taskRoot + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) { throw '采集输出必须在本工作树内。' }
$destination = Join-Path $outputAbsolute (Split-Path $SessionPath -Leaf)
New-Item -ItemType Directory -Path $destination -Force | Out-Null
$issues = New-Object 'System.Collections.Generic.List[string]'
if (!$candidateValid) { $issues.Add('候选清单缺失或程序哈希不匹配，需核对实际运行版本。') }
if (@($resourceChecks | Where-Object { !$_.sameAsBuildInput }).Count) { $issues.Add('EXE/PVF/sk.dat 输入身份与准备时不同；须先区分资源变化与协议问题。') }
$copied = @()
foreach ($name in @('events.jsonl', 'client.log', 'helper.err', 'helper.out', 'run.json')) {
    $source = Join-Path $SessionPath $name
    if (!(Test-Path -LiteralPath $source -PathType Leaf)) { $issues.Add('缺少服务端会话文件：' + $name); continue }
    if ((Get-Item -LiteralPath $source).Length -gt 64MB) { $issues.Add('文件超过 64 MiB，保留原位未复制：' + $source); continue }
    Copy-Item -LiteralPath $source -Destination (Join-Path $destination $name)
    $copied += $name
}
foreach ($name in @('adventure-elite.status.json', 'adventure-elite.log', 'adventure-elite-build.json')) {
    $source = Join-Path $PatchDirectory $name
    if (Test-Path -LiteralPath $source -PathType Leaf) { Copy-Item -LiteralPath $source -Destination (Join-Path $destination $name); $copied += $name }
}
$native = New-Object 'System.Collections.Generic.List[object]'
$traceName = [string]$status.nativeTraceFile
if ($traceName -match '^adventure-elite-native-\d+-\d+\.jsonl$') {
    $tracePath = Join-Path $PatchDirectory $traceName
    if (Test-Path -LiteralPath $tracePath -PathType Leaf) {
        Copy-Item -LiteralPath $tracePath -Destination (Join-Path $destination $traceName)
        $copied += $traceName
        $native = Read-Lines (Join-Path $destination $traceName) $issues
    } else { $issues.Add('本次进程没有原生通知日志。') }
} else { $issues.Add('状态不含本版按进程命名的日志，不能将旧日志当作本次。') }
foreach ($row in $native) {
    if ($row.exceptionCode -gt 0) { $issues.Add('原生处理异常：code=' + $row.exceptionCode + ' ip=' + $row.exceptionIp); break }
}
$clientText = if (Test-Path -LiteralPath (Join-Path $destination 'client.log')) { (Read-SharedText (Join-Path $destination 'client.log')) } else { '' }
$pidMatch = $status.processId -and $clientText -match ('(?m)ROOT_PID\s+' + [regex]::Escape([string]$status.processId) + '\s*$')
if (!$pidMatch) { $issues.Add('客户端 ROOT_PID 与 DLL 状态进程不一致。') }
if ($status.version -ne $build.version -or !$status.ordinaryPrepareReady) { $issues.Add('本次 DLL 版本与候选不匹配或未准备完成；检查初始化失败位置。') }
foreach ($row in $native) {
    if ($row.processId -ne $status.processId -or $row.runTick -ne $status.runTick) { $issues.Add('原生日志含其它进程或其它启动代次。'); break }
}
if (@($native | Where-Object { $_.traceDropped -gt 0 -or $_.phase -eq 'trace-limit' }).Count) { $issues.Add('原生日志有丢弃或达到采集上限，不能声称采集完整。') }
foreach ($group in ($native | Where-Object { $_.phase -ne 'trace-limit' } | Group-Object sequence)) {
    $phases = @($group.Group | ForEach-Object { $_.phase })
    if ('arrival' -notin $phases -or 'before' -notin $phases -or 'after' -notin $phases -or 'scope-closed' -notin $phases) { $issues.Add('原生 sequence ' + $group.Name + ' 缺少处理阶段，可能异常退出或未完成采集。') }
}
$lifecycle = New-Object 'System.Collections.Generic.List[object]'
$lifeName = [string]$status.lifecycleTraceFile
if ($status.version -in @('0.3.2', '0.3.3', '0.3.4','0.3.5','0.3.6','0.3.7','0.3.8','0.3.9','0.3.10')) {
    if (!$status.ordinaryLifecycleReady) { $issues.Add('生命周期采样未初始化。') }
    if ($lifeName -match '^adventure-elite-lifecycle-\d+-\d+\.jsonl$' -and (Test-Path -LiteralPath (Join-Path $PatchDirectory $lifeName))) {
        Copy-Item -LiteralPath (Join-Path $PatchDirectory $lifeName) -Destination (Join-Path $destination $lifeName)
        $copied += $lifeName
        $lifecycle = Read-Lines (Join-Path $destination $lifeName) $issues
        foreach ($row in $lifecycle) {
            if ($row.processId -ne $status.processId -or $row.runTick -ne $status.runTick -or $row.traceDropped -gt 0 -or $row.traceLimit) {
                $issues.Add('生命周期日志进程/代次不匹配或有丢弃/上限。'); break
            }
        }
    } elseif ($native.Count -gt 0) { $issues.Add('缺少本次生命周期采样日志；不得据此推断返回后存活。') }
}
$ownerSamples = @()
if ($status.version -in @('0.3.3','0.3.4','0.3.5','0.3.6','0.3.7','0.3.8','0.3.9','0.3.10')) {
    if ($status.lifecycleTraceVersion -ne 2) { $issues.Add('身份/主人采样版本不符。') }
    foreach ($row in $lifecycle) {
        $shapeValid = $row.traceVersion -eq 2
        foreach ($field in @('identityWords', 'identityValid', 'objectId', 'ownerStrong', 'ownerKind', 'ownerObjectId', 'ownerControllerId', 'ownerIdentityValid', 'sceneAttached', 'ownerMatchesCurrentPlayer', 'weakStrong')) {
            if (@($row.$field).Count -ne 3) { $shapeValid = $false }
        }
        if (!$shapeValid) { $issues.Add('身份/主人采样字段缺失或版本错误。'); continue }
        $valid = $row.available -and $row.consistent -and $row.identityMatches -and $row.weakAlive -gt 0
        $ids = @()
        for ($slot = 0; $slot -lt 3; $slot++) {
            if (@($row.identityWords[$slot]).Count -ne 2) { $valid = $false; continue }
            if ($row.weakStrong[$slot] -le 0) { continue }
            if ($row.identityValid[$slot] -ne 1 -or $row.objectId[$slot] -lt 0 -or
                $row.ownerStrong[$slot] -le 0 -or $row.ownerIdentityValid[$slot] -ne 1 -or $row.ownerMatchesCurrentPlayer[$slot] -ne 1 -or
                $row.ownerObjectId[$slot] -lt 0 -or $row.ownerControllerId[$slot] -ne $row.currentOwner -or
                $row.objectId[$slot] -eq $row.ownerObjectId[$slot] -or $row.objectId[$slot] -in $ids) { $valid = $false }
            $ids += $row.objectId[$slot]
        }
        # Field equality is a diagnostic observation, not an authorization or combat verdict.
        if ($valid) { $ownerSamples += $row }
    }
}
$entry = New-Object 'System.Collections.Generic.List[object]'
if ($status.version -in @('0.3.4','0.3.5','0.3.6','0.3.7','0.3.8','0.3.9','0.3.10')) {
    if (!$status.ordinaryEntryTraceReady) { $issues.Add('首房只读观察钩子未初始化：' + $status.entryFailureSite) }
    $entryName = [string]$status.entryTraceFile
    if ($entryName -notmatch '^adventure-elite-entry-\d+-\d+\.jsonl$') { $issues.Add('首房日志名无效。') }
    elseif (Test-Path -LiteralPath (Join-Path $PatchDirectory $entryName)) {
        Copy-Item -LiteralPath (Join-Path $PatchDirectory $entryName) -Destination (Join-Path $destination $entryName)
        $copied += $entryName
        $entry = Read-Lines (Join-Path $destination $entryName) $issues
        foreach ($row in $entry) {
            if ($row.processId -ne $status.processId -or $row.runTick -ne $status.runTick -or $row.traceVersion -ne 1 -or $row.traceDropped -gt 0 -or $row.traceLimit -or $row.battleEnabled) {
                $issues.Add('首房日志进程/代次/版本不匹配、有丢弃/上限或出现启用战斗标记。'); break
            }
            foreach ($field in @('objectId','ownerMatchesCurrentPlayer','controllerId','vtable','ownerGetter','combatGetter','sceneVectorMatches')) {
                if (@($row.$field).Count -ne 3) { $issues.Add('首房日志字段缺失：' + $field) }
            }
        }
    }
}
$events = Read-Lines (Join-Path $destination 'events.jsonl') $issues
$related = @($events | Where-Object { $_.id -in @(15, 16, 37, 38, 39, 40, 42, 43, 45, 46, 69, 70, 71, 72, 117, 2015, 2062, 132, 1395, 1719, 1754, 1811, 1382, 1879) -or $_.kind -in @('channel_identity_sent', 'character_mode_projection', 'adventure_elite_entry_probe', 'adventure_elite_combat_request', 'adventure_elite_dispatch_committed', 'dungeon_request_refused', 'monster_death_ack', 'monster_death_confirmed', 'drop_roll_skipped') })
$eventFile = Join-Path $destination 'elite-events.jsonl'
$text = (@($related | ForEach-Object { $_ | ConvertTo-Json -Compress -Depth 20 }) -join [Environment]::NewLine)
[IO.File]::WriteAllText($eventFile, $text + [Environment]::NewLine, $utf8)
$requests = @($related | Where-Object { $_.id -eq 1811 -and $_.kind -eq 'client_frame' })
$decisions = @($related | Where-Object { $_.kind -eq 'adventure_elite_transaction_decision' })
if ($requests.Count -gt 0 -and @($decisions | Where-Object { $_.id -eq 1811 }).Count -eq 0) { $issues.Add('有 CMD1811 但没有新版服务端决策日志。') }
$done = @($native | Where-Object { $_.opcode -eq 1879 -and $_.phase -eq 'after' -and $_.readerSuccess -eq 1 -and $_.weakAlive -gt 0 })
$latestNative = $native | Where-Object { $_.phase -eq 'after' -and $_.readerSuccess -eq 1 } | Select-Object -Last 1
$cloneReleased = $done.Count -gt 0 -and $latestNative.sequence -gt $done[-1].sequence -and $latestNative.weakAlive -eq 0
$progress = if (!$pidMatch -or $status.version -ne $build.version) { 'different-or-old-client-run' } elseif (!$status.ordinaryPrepareReady) { 'adapter-initialization-failed' } elseif (!$requests.Count) { 'no-native-load-request' } elseif (!$done.Count) { 'load-request-seen-clone-not-yet-confirmed' } elseif ($cloneReleased) { 'client-clone-observed-then-released' } else { 'client-clone-observed-battle-unverified' }
$selectionOpened = $false
$selectionReturned = $false
foreach ($event in $events) {
    if ($event.kind -eq 'client_frame' -and $event.id -eq 15) { $selectionOpened = $true }
    if ($selectionOpened -and $event.kind -eq 'client_frame' -and $event.id -eq 132) { $selectionReturned = $true }
}
$lifeValid = @($lifecycle | Where-Object { $_.processId -eq $status.processId -and $_.runTick -eq $status.runTick -and $_.available -and $_.consistent -and $_.identityMatches -and !$_.traceLimit })
$lifeLast = $lifeValid | Select-Object -Last 1
$lifeReleased = $done.Count -gt 0 -and @($lifeValid | Where-Object { $_.tick -ge $done[-1].tick -and $_.weakAlive -eq 0 }).Count -gt 0
if ($lifeReleased) { $progress = 'client-clone-observed-then-released' }
# 143C76780 sends CMD1395 at these two proven flush return addresses.
# Map only when the complete server/request and native/caller counts agree.
$uiFlushRvas = @(0x3C7682A, 0x3C768AF)
$lifeUi = @($lifecycle | Where-Object { $_.cause -eq 'native-flush' -and $_.callerRva -in $uiFlushRvas })
$serverUi = @()
$completed = $false
$returnAccepted = $false
foreach ($event in $events) {
    if ($event.kind -eq 'adventure_elite_packet_sent' -and $event.id -eq 1879) { $completed = $true }
    if ($event.kind -eq 'dungeon_selection_return') { $returnAccepted = $true }
    if ($completed -and $event.kind -eq 'client_frame' -and $event.id -eq 1395) { $serverUi += [bool]$returnAccepted }
}
$postReturn = $null
if ($done.Count -gt 0 -and $selectionReturned -and $returnAccepted -and $lifeUi.Count -gt 0 -and $lifeUi.Count -eq $serverUi.Count -and $issues.Count -eq 0) {
    for ($i = 0; $i -lt $lifeUi.Count; $i++) {
        $row = $lifeUi[$i]
        if ($serverUi[$i] -and $row -in $lifeValid -and $row.weakAlive -eq $done[-1].weakAlive) {
            $bindings = $true
            for ($slot = 0; $slot -lt 3; $slot++) {
                if ($row.weakStrong[$slot] -gt 0 -and ($row.actorKind[$slot] -ne 5 -or
                    $row.controllerBound[$slot] -ne 1 -or $row.controllerId[$slot] -lt 0 -or
                    $row.controllerId[$slot] -ne $row.controllerWire[$slot])) { $bindings = $false }
            }
            if ($bindings) { $postReturn = $row }
        }
    }
}
$entryDecision = @($events | Where-Object { $_.kind -eq 'adventure_elite_entry_probe' -and $_.accepted -and !$_.battle_enabled })
$entryProbeObservations=@($events | Where-Object {$_.kind -eq 'adventure_elite_entry_probe'})
$specialElitePackets=@($events | Where-Object {$_.kind -eq 'adventure_elite_special_packet_sent'})
$moonEliteEntries=@($entryProbeObservations | Where-Object {$_.candidate_stage -eq 'moon-solo'})
$odysseyEntryObservations=@($entryProbeObservations | Where-Object {$_.odyssey -eq $true -or $_.requested_source_odyssey -eq $true})
$odysseyFlowObservations=@($events | Where-Object {$_.kind -match 'odyssey' -or $_.name -match 'odyssey' -or $_.id -in @(2856)})
$storyEntryObservations=@($entryProbeObservations | Where-Object {$_.candidate_stage -eq 'ordinary-story' -or $_.requested_quest -gt 0})
# CMD72 retry creates a run without another CMD16. Keep its original server
# serial and correlate the successful commit; do not invent an entry decision.
$retryTransitions=@($events | Where-Object {$_.kind -eq 'adventure_elite_dispatch_committed' -and $_.id -eq 72 -and $_.after.active -eq $true})
$retryEntries=@()
foreach($transition in $retryTransitions) {
    $prior=@($events | Where-Object {$_.kind -eq 'adventure_elite_combat_request' -and $_.id -eq 72 -and $_.before.run_id -eq $transition.origin_run_id -and $_.entry_serial -eq $transition.entry_serial})
    $valid=$prior.Count -eq 1 -and $prior[0].processed -and $prior[0].before.completed -eq $true -and $prior[0].before.result_sent -eq $true -and @($prior[0].packets | Where-Object {$_.id -eq 16 -and $_.kind -eq 1}).Count -eq 1
    $a=$transition.after
    $clean=$a.run_id -and $a.run_id -ne $transition.origin_run_id -and $a.loaded -eq $false -and $a.completed -eq $false -and $a.death_sent_count -eq 0 -and $a.drops_active -eq $false -and $a.cards_active -eq $false -and $a.result_sent -eq $false -and $a.completion_sent -eq $false
    if(!$valid -or !$clean){$issues.Add('再次挑战缺原请求/入场计划/成功提交后的干净新RunID。');continue}
    $retryEntries+=$transition
}
# A direct move is a real entry decision plus a successful transport commit.
# Preserve distinct origin/new run generations; never call it a town return.
$directMoveTransitions=@($events | Where-Object {$_.kind -eq 'adventure_elite_dispatch_committed' -and $_.id -eq 2062 -and $_.after.active -eq $true})
$directMoveRuns=@()
foreach($transition in $directMoveTransitions) {
    $prior=@($events | Where-Object {$_.kind -eq 'adventure_elite_combat_request' -and $_.id -eq 2062 -and $_.before.run_id -eq $transition.origin_run_id -and $_.entry_serial -eq $transition.entry_serial})
    $a=$transition.after
    $valid=$prior.Count -eq 1 -and $prior[0].processed -and $prior[0].before.completed -eq $true -and $prior[0].before.result_sent -eq $true -and $prior[0].before.source_odyssey -eq $true -and $prior[0].before.requested_source_odyssey -eq $true -and $transition.entry_serial -eq $prior[0].before.entry_serial+1 -and $prior[0].pending_run_id -eq $a.run_id -and $prior[0].pending_dungeon -eq $a.dungeon -and $prior[0].before.requested_dungeon -eq $a.dungeon
    $ids=@($prior | ForEach-Object {$_.packets} | ForEach-Object {$_.id})
    $plan=($ids.Count -ge 5 -and $ids[0] -eq 15 -and $ids[1] -eq 27 -and $ids[2] -eq 16 -and $ids -contains 28 -and $ids -contains 29 -and $ids -notcontains 2062)
    $clean=$a.run_id -and $a.run_id -ne $transition.origin_run_id -and $a.loaded -eq $false -and $a.completed -eq $false -and $a.death_sent_count -eq 0 -and $a.drops_active -eq $false -and $a.cards_active -eq $false -and $a.result_sent -eq $false -and $a.completion_sent -eq $false
    if(!$valid -or !$plan -or !$clean){$issues.Add('奥德赛直进缺原请求/源目标/入场计划/递增代次或干净新RunID。')}
    $directMoveRuns+=[pscustomobject]@{originRunId=$transition.origin_run_id;runId=$a.run_id;dungeon=$a.dungeon;entrySerial=$transition.entry_serial;requestAndSourceMatched=[bool]$valid;entryPlanMatched=[bool]$plan;cleanNewRun=[bool]$clean}
}
$freshServerEntries=@($events | Where-Object {($_.kind -eq 'adventure_elite_entry_probe' -and $_.accepted -and !$_.battle_enabled) -or $_ -in $retryEntries})
$entryBefore = @($entry | Where-Object { $_.phase -eq 'loader-before' })
$entryAfter = @($entry | Where-Object { $_.phase -eq 'loader-after' })
$entryGates = @($entry | Where-Object { $_.phase -eq 'registration-gate' -and $_.callerRva -eq 0x5B25191 })
if ($status.version -in @('0.3.4','0.3.5','0.3.6','0.3.7','0.3.8','0.3.9','0.3.10') -and $entryDecision.Count -gt 0 -and (!$entryBefore.Count -or !$entryAfter.Count -or !$entryGates.Count)) { $issues.Add('首房已获服务端许可，但缺加载前后或精确注册门禁样本。') }
if ($entryBefore.Count -ne $entryAfter.Count) { $issues.Add('首房加载前后样本不配对，不能声称完整。') }
$registrationScope = @($entry | Where-Object { $_.phase -in @('registration-scope-allowed','registration-scope-rejected','registration-scope-native') })
$registrationAllowed = @($registrationScope | Where-Object { $_.phase -eq 'registration-scope-allowed' })
$registrationChannel = @($entry | Where-Object { $_.phase -eq 'registration-channel-native' -and $_.callerRva -eq 0x5B251C3 })
$registrationResults = @($entry | Where-Object { $_.phase -eq 'native-registration-result' -and $_.callerRva -eq 0x5B2529D })
if ($status.version -in @('0.3.5','0.3.6','0.3.7','0.3.8','0.3.9','0.3.10')) {
    if (!$status.ordinaryRegistrationReady) { $issues.Add('首房登记适配未初始化：' + $status.registrationFailureSite) }
    foreach ($row in $entry) {
        if (@($row.registrationChecks).Count -ne 11 -or @($row.registrationPlayerRef).Count -ne 2 -or @($row.registrationNativePlayerRef).Count -ne 2) { $issues.Add('首房登记条件/真人引用字段不完整。'); break }
    }
    if ($entryDecision.Count -gt 0 -and !$registrationScope.Count) { $issues.Add('首房许可后缺少登记范围判定。') }
    foreach ($row in $registrationAllowed) {
        $checks=@($row.registrationChecks)
        if ($row.callerRva -ne 0x5B251AE -or $checks.Count -ne 11 -or @($checks | Select-Object -First 10 | Where-Object { $_ -ne 1 }).Count -gt 0 -or $checks[10] -lt 1 -or $checks[10] -gt 3 -or $row.nativeResult -ne 1) { $issues.Add('登记放行条件未全部匹配。') }
        $ends=@($entryAfter | Where-Object {$_.thread -eq $row.thread -and $_.ordinal -gt $row.ordinal})
        $begins=@($entryBefore | Where-Object {$_.thread -eq $row.thread -and $_.ordinal -lt $row.ordinal})
        if(!$ends.Count -or !$begins.Count) { $issues.Add('登记范围没有被同线程加载前后样本包围。') }
        if (@($row.registrationPlayerRef).Count -eq 2 -and @($row.registrationNativePlayerRef).Count -eq 2 -and ($row.registrationPlayerRef[0] -eq 0 -or $row.registrationPlayerRef[1] -eq 0 -or $row.registrationPlayerRef[0] -ne $row.registrationNativePlayerRef[0] -or $row.registrationPlayerRef[1] -ne $row.registrationNativePlayerRef[1])) { $issues.Add('登记放行的真人原生引用不一致。') }
        $endOrdinal=if($ends.Count){$ends[0].ordinal}else{[long]::MaxValue}
        $channels=@($registrationChannel | Where-Object { $_.thread -eq $row.thread -and $_.ordinal -gt $row.ordinal -and $_.ordinal -lt $endOrdinal })
        $results=@($registrationResults | Where-Object { $_.thread -eq $row.thread -and $_.ordinal -gt $row.ordinal -and $_.ordinal -lt $endOrdinal })
        if ($channels.Count -and $results.Count -and ($channels[0].ordinal -ge $results[0].ordinal -or ($ends.Count -and $results[-1].ordinal -ge $ends[0].ordinal))) { $issues.Add('登记频道/原生结果/加载结束次序错误。') }
        if (!$channels.Count -or !$results.Count) { $issues.Add('登记范围已放行但缺同线程频道门禁/原生登记结果。') }
    }
}
# v0.3.9: correlate the fresh-dungeon loader and exact active getter consumer.
$activeReentry=@($entry | Where-Object {$_.phase -eq 'registration-active-reentry'})
$freshRegistrationCycles=@()
if($status.version -in @('0.3.9','0.3.10')) {
    foreach($row in $entry) {
        if($null -eq $row.PSObject.Properties['loaderCallerRva'] -or $null -eq $row.PSObject.Properties['freshDungeonEntry']){$issues.Add('重复登记缺原生加载调用者字段。');break}
    }
    foreach($row in $activeReentry) {
        if($row.callerRva -ne 0x5B251D8 -or $row.loaderCallerRva -ne 0x6D2A1E7 -or !$row.freshDungeonEntry -or $row.nativeResult -ne 1 -or $row.manager576 -ne 1 -or @($row.registrationChecks | Select-Object -First 10 | Where-Object {$_ -ne 1}).Count -gt 0){$issues.Add('重复登记active适配缺精确新副本路径或稳定范围。')}
    }
    foreach($begin in @($entryBefore | Where-Object {$_.callerRva -eq 0x6D2A1E7})) {
        $end=@($entryAfter | Where-Object {$_.thread -eq $begin.thread -and $_.ordinal -gt $begin.ordinal} | Select-Object -First 1)
        $endOrdinal=if($end.Count){$end[0].ordinal}else{[long]::MaxValue}
        $window=@($entry | Where-Object {$_.thread -eq $begin.thread -and $_.ordinal -gt $begin.ordinal -and $_.ordinal -lt $endOrdinal})
        $scope=@($window | Where-Object {$_.phase -eq 'registration-scope-allowed'})
        $active=@($window | Where-Object {$_.phase -eq 'registration-active-reentry'})
        $channels=@($window | Where-Object {$_.phase -eq 'registration-channel-native'})
        $results=@($window | Where-Object {$_.phase -eq 'native-registration-result'})
        $registered=$end.Count -eq 1 -and $scope.Count -eq 1 -and $channels.Count -eq 1 -and $results.Count -eq $begin.weakAlive -and $results.Count -gt 0 -and @($results | Where-Object {$_.nativeResult -ne 1}).Count -eq 0
        if($registered){foreach($index in 0..2){if($begin.objectId[$index] -ge 0 -and $end[0].sceneVectorMatches[$index] -ne 1){$registered=$false}}}
        $adapterValid=($begin.manager576 -eq 0 -and !$active.Count) -or ($begin.manager576 -eq 1 -and $active.Count -eq 1 -and $channels.Count -eq 1 -and $results.Count -gt 0 -and $channels[0].ordinal -lt $active[0].ordinal -and $active[0].ordinal -lt $results[0].ordinal)
        if(!$begin.freshDungeonEntry -or $begin.loaderCallerRva -ne $begin.callerRva -or !$registered -or !$adapterValid){$issues.Add('新副本加载缺完整登记/新场景成员或active消费顺序。')}
        $freshRegistrationCycles+=[pscustomobject]@{loaderOrdinal=$begin.ordinal;endOrdinal=$(if($end.Count){$end[0].ordinal}else{0});thread=$begin.thread;managerActive=$begin.manager576;companions=$begin.weakAlive;nativeResults=$results.Count;activeAdaptations=$active.Count;allRegistered=[bool]$registered;adapterValid=[bool]$adapterValid}
    }
    if($freshRegistrationCycles.Count -ne $freshServerEntries.Count){$issues.Add('服务端入场与客户端新副本加载次数不匹配。')}
}
# Read-only provenance: never infer a human killer from 0xffff or sample counts alone.
$combat = New-Object 'System.Collections.Generic.List[object]'
$combatIssues = New-Object 'System.Collections.Generic.List[string]'
$combatLinks = @()
$deathPackets = @()
if ($status.version -in @('0.3.7','0.3.8','0.3.9','0.3.10')) {
    if (!$status.ordinaryCombatTraceReady -or $status.combatTraceVersion -ne $(if($status.version -in @('0.3.8','0.3.9','0.3.10')){2}else{1})) { $combatIssues.Add('战斗归属观察未初始化或版本不符。') }
    $combatName = [string]$status.combatTraceFile
    if ($combatName -notmatch '^adventure-elite-combat-\d+-\d+\.jsonl$' -or !(Test-Path -LiteralPath (Join-Path $PatchDirectory $combatName))) { $combatIssues.Add('缺少本次战斗归属日志。') }
    else {
        Copy-Item -LiteralPath (Join-Path $PatchDirectory $combatName) -Destination (Join-Path $destination $combatName)
        $copied += $combatName
        $combat = Read-Lines (Join-Path $destination $combatName) $combatIssues
    }
    $seenTransactions = @{}
    $lastOrdinal=0
    foreach ($row in $combat) {
        $shape = $true
        foreach ($field in @('transaction','parent','callerRva','completed','victimId','sourceId','sourceController','eliteSlot','ownerId','ownerController','ownerMatches','nativeWritesChanged','battleEnabled')) {
            if ($null -eq $row.PSObject.Properties[$field]) { $shape=$false }
        }
        foreach ($field in @('readable','stable','killer','sourceField','playerRef')) { if (@($row.$field).Count -ne 2) { $shape=$false } }
        if (@($row.sourceRef).Count -ne 3 -or !$shape) { $combatIssues.Add('战斗日志字段不完整。'); continue }
        if ($row.processId -ne $status.processId -or $row.runTick -ne $status.runTick -or $row.traceVersion -ne $status.combatTraceVersion -or $row.traceDropped -gt 0 -or $row.traceLimit -or $row.battleEnabled) { $combatIssues.Add('战斗日志进程/代次/版本不符、丢弃/上限或出现运行改写标记。') }
        if ($row.ordinal -ne $lastOrdinal+1 -or $seenTransactions.ContainsKey([string]$row.transaction)) { $combatIssues.Add('战斗日志序号缺失或事务重复。') }
        $lastOrdinal=$row.ordinal; $seenTransactions[[string]$row.transaction]=$true
        if (!$row.completed) { $combatIssues.Add('战斗原生调用未正常返回。') }
        if($status.version -in @('0.3.8','0.3.9','0.3.10')) {
            foreach($field in @('effectiveArgument','uniqueSource','controllerBound','controllerWire','combatGetterRva','roomSerial')) { if($null -eq $row.PSObject.Properties[$field]){$combatIssues.Add('归属候选新增字段缺失：'+$field)} }
            if($row.nativeWritesChanged -and ($row.phase -ne 'killer-write' -or $row.victimKind -ne 3 -or $row.eliteSlot -lt 0 -or $row.eliteSlot -ge 3 -or $row.readable[0] -ne 1 -or $row.stable[0] -ne 1 -or $row.sourceKind -ne 5 -or $row.sourceStrong -le 0 -or $row.sourceField[1] -ne $row.sourceId -or $row.callerRva -ne 0x5DD022B -or $row.parent -le 0 -or $row.argument -ne 65535 -or $row.effectiveArgument -ne $row.ownerId -or $row.ownerId -ne $row.ownerController -or $row.ownerMatches -ne 1 -or !$row.uniqueSource -or !$row.controllerBound -or $row.controllerWire -ne 65535 -or $row.sourceController -ne 65535 -or $row.combatGetterRva -ne 0x14CC20)) { $combatIssues.Add('击杀者改写缺少精确原生来源或主人条件。') }
        } elseif($row.nativeWritesChanged) { $combatIssues.Add('只读观察版本出现改写标记。') }
        if ($row.phase -notin @('source-assignment','killer-write','death-send')) { $combatIssues.Add('未知战斗观察阶段。') }
        if (0 -in $row.readable -or 0 -in $row.stable -or $row.victimId -lt 0) { $combatIssues.Add('战斗对象读取不完整、不稳定或身份无效。') }
        if ($row.phase -eq 'killer-write' -and $row.killer[1] -ne ([uint32]([int64]$(if($status.version -in @('0.3.8','0.3.9','0.3.10')){$row.effectiveArgument}else{$row.argument}) -band 0xffffffffL))) { $combatIssues.Add('原生击杀者写入结果与参数不符。') }
    }
    foreach ($row in @($combat | Where-Object { $_.phase -eq 'killer-write' -and $_.parent -gt 0 -and $_.callerRva -eq 0x5DD022B })) {
        $parents=@($combat | Where-Object { $_.phase -eq 'source-assignment' -and $_.transaction -eq $row.parent -and $_.victim -eq $row.victim -and $_.thread -eq $row.thread -and $_.input -eq $row.input -and $_.ordinal -gt $row.ordinal })
        if ($parents.Count -ne 1) { $combatIssues.Add('归属写入缺同线程同受害对象的原生来源父调用。'); continue }
        if ($row.eliteSlot -ge 0 -and $row.eliteSlot -lt 3 -and $row.ownerMatches -eq 1 -and $row.sourceKind -eq 5 -and $row.sourceStrong -gt 0 -and $row.sourceId -ge 0 -and $row.sourceField[1] -eq $row.sourceId -and $row.argument -eq $row.sourceController -and $row.completed -and 0 -notin $row.stable) {
            $combatLinks += [pscustomobject]@{ transaction=$row.transaction; parent=$row.parent; victimId=$row.victimId; sourceId=$row.sourceId; slot=$row.eliteSlot; controller=$row.sourceController; owner=$row.ownerId; ownerController=$row.ownerController; callerRva=$row.callerRva }
        }
    }
    foreach ($event in @($events | Where-Object { $_.kind -eq 'client_frame' -and $_.id -eq 39 })) {
        $hex=[string]$event.plain_hex
        if ($hex -notmatch '^[0-9a-fA-F]+$' -or $hex.Length%16 -ne 0 -or $hex.Length -lt 128 -or $hex.Length -gt 8192) { $combatIssues.Add('CMD39 原生明文缺失或长度不符合既有 reader。'); continue }
        $participantCount=[Convert]::ToUInt32($hex.Substring(60,2),16)
        if (31+$participantCount*14+7 -gt $hex.Length/2) { $combatIssues.Add('CMD39 既有参与者边界无效。'); continue }
        $entity=[Convert]::ToUInt32(($hex.Substring(6,2)+$hex.Substring(4,2)+$hex.Substring(2,2)+$hex.Substring(0,2)),16)
        $killer=[Convert]::ToUInt32(($hex.Substring(10,2)+$hex.Substring(8,2)),16)
        $deathPackets += [pscustomobject]@{entity=$entity;killer=$killer;time=$event.time}
    }
    $senders=@($combat | Where-Object { $_.phase -eq 'death-send' -and $_.completed })
    if (!$combatLinks.Count) { $combatIssues.Add('尚无精锐攻击来源到原生击杀者写入的完整动态关联。') }
    if (!$senders.Count -or !$deathPackets.Count) { $combatIssues.Add('缺少原生死亡发送或服务端 CMD39 样本。') }
    if ($senders.Count -ne $deathPackets.Count) { $combatIssues.Add('原生死亡发送与服务端 CMD39 数量不一致，不能声称覆盖完整。') }
    else {
        for($i=0;$i -lt $senders.Count;$i++) {
            if($senders[$i].victimId -ne $deathPackets[$i].entity -or ($senders[$i].killer[0] -band 65535) -ne $deathPackets[$i].killer) { $combatIssues.Add('死亡发送与 CMD39 按序身份/击杀者不一致。'); break }
        }
    }
    foreach ($issue in $combatIssues) { $issues.Add($issue) }
}
$ownedWrites=@($combat | Where-Object { $_.phase -eq 'killer-write' -and $_.nativeWritesChanged })
$serverCombat=@($events | Where-Object { $_.kind -eq 'adventure_elite_combat_request' })
$serverDeaths=@($serverCombat | Where-Object { $_.id -eq 39 -and $_.processed })
$freshPlayerDeaths=@($serverDeaths | Where-Object { $_.before.dead -eq $false -and $_.after.dead -eq $true -and $_.after.unowned -eq $false -and $_.after.killer -eq $_.owner_wire_id })
# Pair requests by stream order, then match the precise source and room generation.
# Player kills are valid ordinary deaths but are not APC ownership evidence.
$ownedFresh=@(); $requestIndex=-1
$senders=@($combat | Where-Object { $_.phase -eq 'death-send' -and $_.completed })
foreach($event in $events) {
    if($event.kind -eq 'client_frame' -and $event.id -eq 39){$requestIndex++}
    if($event.kind -ne 'adventure_elite_combat_request' -or $event.id -ne 39 -or !$event.processed -or $event.before.dead -ne $false -or $event.after.dead -ne $true -or $event.after.unowned -ne $false -or $event.after.killer -ne $event.owner_wire_id){continue}
    if($requestIndex -lt 0 -or $requestIndex -ge $senders.Count){continue}
    $sender=$senders[$requestIndex]
    $matches=@($ownedWrites | Where-Object { $_.victimId -eq $event.after.entity -and $_.effectiveArgument -eq $event.after.killer -and $_.roomSerial -eq $sender.roomSerial -and $_.ordinal -lt $sender.ordinal -and ($_.sourceRef -join ',') -eq ($sender.sourceRef -join ',') -and $_.sourceId -eq $sender.sourceId })
    if($matches.Count){$ownedFresh+=$event}
}
$ownedIssues=New-Object 'System.Collections.Generic.List[string]'
if($status.version -in @('0.3.8','0.3.9','0.3.10')) {
    if(!$ownedWrites.Count){$ownedIssues.Add('尚无符合来源条件的精锐击杀者改写样本。')}
    if(!$serverCombat.Count -or !$ownedFresh.Count){$ownedIssues.Add('缺服务端归属候选处理或归属新死亡记录。')}
    foreach($row in $serverCombat) {
        if($row.candidate_version -notin @('0.3.8','0.3.10','0.3.11','0.3.12','0.3.13','0.3.14','0.3.15','0.3.16','0.3.17','0.3.18','0.3.19') -or $row.frozen_owner -ne $row.character_id -or $row.frozen_channel -ne $row.channel_type -or $row.client_acceptance -ne 'pending'){$ownedIssues.Add('服务端候选版本/冻结身份字段不符。')}
    }
    foreach($issue in $ownedIssues){$issues.Add($issue)}
}
# Entry serial is a server generation, distinct from the DLL's per-room generation.
$reentryDecisions=@($entryDecision | Where-Object {$_.candidate_stage -in @('ordinary-reentry','ordinary-story','ordinary-odyssey')})
$reentryRuns=@(); $seenRuns=@{}; $previousSerial=0
foreach($decision in $reentryDecisions) {
    if(!$decision.run_id -or $seenRuns.ContainsKey([string]$decision.run_id) -or $decision.entry_serial -ne $previousSerial+1){$issues.Add('重复入场的RunID/递增序号不符。')}
    if($decision.run_id){$seenRuns[[string]$decision.run_id]=$true}; $previousSerial=$decision.entry_serial
    $entryCommitted=@($events | Where-Object {$_.kind -eq 'adventure_elite_dispatch_committed' -and $_.id -in @(16,2062) -and $_.after.run_id -eq $decision.run_id -and $_.entry_serial -eq $decision.entry_serial -and $_.after.death_sent_count -eq 0 -and $_.after.drops_active -eq $false -and $_.after.cards_active -eq $false -and $_.after.result_sent -eq $false -and $_.after.completion_sent -eq $false})
    if($entryCommitted.Count -ne 1){$issues.Add('重复入场缺成功发送后的独立run或初始状态不干净。')}
    $rows=@($serverCombat | Where-Object {$_.before.run_id -eq $decision.run_id})
    $committed=@($events | Where-Object {$_.kind -eq 'adventure_elite_dispatch_committed' -and $_.origin_run_id -eq $decision.run_id})
    $returns=@($committed | Where-Object {$_.id -in @(42,72) -and $_.after.active -eq $false -and $_.after.pending_town_arrival -eq $false -and $_.after.death_sent_count -eq 0 -and $_.after.drops_active -eq $false -and $_.after.cards_active -eq $false -and $_.after.result_sent -eq $false -and $_.after.completion_sent -eq $false})
    foreach($row in $rows){
        $direct=$row.id -eq 2062 -and $row.processed -and $row.pending_run_id -and $row.pending_run_id -ne $decision.run_id
        $expected=if($direct){$decision.entry_serial+1}else{$decision.entry_serial}
        if($row.entry_serial -ne $expected -or $row.before.entry_serial -ne $decision.entry_serial){$issues.Add('副本请求串入其它入场代次。')}
    }
    foreach($row in $committed){
        $direct=$row.id -eq 2062 -and $row.after.run_id -and $row.after.run_id -ne $decision.run_id
        $expected=if($direct){$decision.entry_serial+1}else{$decision.entry_serial}
        if($row.entry_serial -ne $expected){$issues.Add('提交后状态串入其它入场代次。')}
    }
    $reentryRuns += [pscustomobject]@{runId=$decision.run_id;entrySerial=$decision.entry_serial;dungeon=$decision.dungeon;sourceScript=$decision.source_script;sourceSha256=$decision.source_sha256;entryCommitted=($entryCommitted.Count -eq 1);ownedDeaths=@($ownedFresh | Where-Object {$_.before.run_id -eq $decision.run_id}).Count;deaths=@($rows | Where-Object {$_.id -eq 39 -and $_.processed}).Count;moves=@($rows | Where-Object {$_.id -eq 45 -and $_.processed}).Count;pickups=@($rows | Where-Object {$_.id -eq 43 -and $_.processed}).Count;results=@($rows | Where-Object {$_.id -eq 46 -and $_.processed}).Count;cards=@($rows | Where-Object {$_.id -in @(69,70,71) -and $_.processed}).Count;returnCommitted=($returns.Count -gt 0);refusals=@($rows | Where-Object {!$_.processed})}
}
$retryRuns=@();$freshRunIds=@{}
for($i=0;$status.version -in @('0.3.9','0.3.10') -and $i -lt $freshServerEntries.Count;$i++) {
    $event=$freshServerEntries[$i];$retry=$event.kind -eq 'adventure_elite_dispatch_committed'
    $runId=if($retry){$event.after.run_id}else{$event.run_id}
    if(!$runId -or $freshRunIds.ContainsKey([string]$runId)){$issues.Add('新副本加载的RunID缺失或重复。')}
    if($runId){$freshRunIds[[string]$runId]=$true}
    if($retry){$retryRuns+=[pscustomobject]@{runId=$runId;originRunId=$event.origin_run_id;serverEntrySerial=$event.entry_serial;freshEntryIndex=$i+1;nativeLoaderOrdinal=$(if($i -lt $freshRegistrationCycles.Count){$freshRegistrationCycles[$i].loaderOrdinal}else{0});ownedDeaths=@($ownedFresh | Where-Object {$_.before.run_id -eq $runId}).Count}}
}
$entryCountValid=($entryDecision.Count -eq 1 -or ($reentryDecisions.Count -gt 0 -and $reentryDecisions.Count -eq $entryDecision.Count))
$entryComplete = $pidMatch -and $candidateValid -and $status.version -in @('0.3.4','0.3.5','0.3.6','0.3.7','0.3.8','0.3.9','0.3.10') -and $entryCountValid -and $entryBefore.Count -gt 0 -and $entryBefore.Count -eq $entryAfter.Count -and $entryGates.Count -gt 0 -and $issues.Count -eq 0
$summary = [ordered]@{
    session = $SessionPath; collectedAt = [DateTime]::UtcNow.ToString('o'); candidateValid = $candidateValid; artifacts = $artifactChecks
    resources = $resourceChecks
    status = $status; pidMatch = [bool]$pidMatch; progress = $progress; requests1811 = $requests.Count
    serverDecisions = $decisions; serverSentPackets = @($related | Where-Object { $_.kind -eq 'adventure_elite_packet_sent' }); nativeRecords = $native.Count; nativeReasons = @($native | Group-Object scopeReason | Select-Object Name, Count)
    nativeCompletion = $done; normalExitObserved = ($clientText -match 'SUMMARY exit=0x0'); copied = $copied; issues = @($issues)
    latestNativeObservation = $latestNative; cloneReleasedAfterCompletion = [bool]($cloneReleased -or $lifeReleased)
    cloneAliveAtLastObservation = ($done.Count -gt 0 -and $(if ($lifeLast -and $lifeLast.tick -ge $latestNative.tick) { $lifeLast.weakAlive -gt 0 } else { $latestNative.weakAlive -gt 0 }))
    coverageComplete = ($pidMatch -and $status.version -eq $build.version -and $native.Count -gt 0 -and $issues.Count -eq 0)
    selectionOpenedObserved = $selectionOpened; selectionReturnObserved = $selectionReturned
    lifecycleRecords = $lifecycle.Count; latestLifecycleObservation = $lifeLast; lifecycleReleaseObserved = $lifeReleased
    selectionReturnAccepted = $returnAccepted; uiFlushSamples = $lifeUi.Count; uiRequestsAfterCompletion = $serverUi.Count
    postReturnCloneVerified = ($null -ne $postReturn); postReturnCloneObservation = $postReturn
    ownerIdentitySamples = $ownerSamples.Count; latestOwnerIdentityObservation = ($ownerSamples | Select-Object -Last 1)
    sceneAttachedObserved = (@($ownerSamples | Where-Object { 1 -in $_.sceneAttached }).Count -gt 0)
    entryProbeDecisions = $entryDecision; entryProbeObservations = $entryProbeObservations; moonEliteEntryObservations=$moonEliteEntries; specialElitePacketObservations=$specialElitePackets; storyEntryObservations = $storyEntryObservations; odysseyEntryObservations = $odysseyEntryObservations; odysseyFlowObservations = $odysseyFlowObservations; odysseyVerified = $false; entryRecords = $entry.Count; entryGateObservations = $entryGates
    entryProbeCoverageComplete = [bool]$entryComplete; latestEntryObservation = ($entryAfter | Select-Object -Last 1)
    nativeRegistrationObserved = (@($entry | Where-Object { $_.phase -eq 'native-registration-result' -and $_.nativeResult -ne 0 }).Count -gt 0)
    registrationScopeObservations = $registrationScope; registrationScopeAccepted = $registrationAllowed.Count -gt 0
    registrationChannelObservations = $registrationChannel; registrationResults = $registrationResults
    registrationActiveReentryObservations = $activeReentry; freshRegistrationCycles = $freshRegistrationCycles
    allFreshEntriesRegistered = ($status.version -in @('0.3.9','0.3.10') -and $freshRegistrationCycles.Count -gt 0 -and @($freshRegistrationCycles | Where-Object {!$_.allRegistered -or !$_.adapterValid}).Count -eq 0 -and $issues.Count -eq 0)
    combatRecords = $combat.Count; combatNativeProvenance = $combatLinks; combatDeathPackets = $deathPackets; combatIssues = @($combatIssues)
    combatCoverageComplete = ($status.version -in @('0.3.7','0.3.8','0.3.9','0.3.10') -and $pidMatch -and $candidateValid -and $combat.Count -gt 0 -and $combatIssues.Count -eq 0 -and $issues.Count -eq 0)
    ownedCombatWrites = $ownedWrites; ownedCombatServerRequests = $serverCombat; ownedCombatFreshDeaths = $ownedFresh
    ownedCombatIssues = @($ownedIssues); ownedCombatProcessedObserved = ($status.version -in @('0.3.8','0.3.9','0.3.10') -and $ownedFresh.Count -gt 0 -and $ownedIssues.Count -eq 0 -and $issues.Count -eq 0)
    ordinaryBatchCoverage = [ordered]@{ deaths=$serverDeaths.Count; pickups=@($serverCombat | Where-Object {$_.id -eq 43 -and $_.processed}).Count; moves=@($serverCombat | Where-Object {$_.id -eq 45 -and $_.processed}).Count; results=@($serverCombat | Where-Object {$_.id -eq 46 -and $_.processed}).Count; cards=@($serverCombat | Where-Object {$_.id -in @(69,70,71) -and $_.processed}).Count; exits=@($serverCombat | Where-Object {$_.id -in @(42,72) -and $_.processed}).Count; refusals=@($serverCombat | Where-Object {!$_.processed}); roomLoadCycles=$entryAfter.Count; nativeRoomSerials=@($combat | Group-Object roomSerial | Select-Object Name,Count) }
    retryRuns = $retryRuns; freshServerEntryCount = $freshServerEntries.Count
    ordinaryReentryRuns = $reentryRuns; directMoveRuns = $directMoveRuns; directMoveRequests = @($related | Where-Object {$_.kind -eq 'client_frame' -and $_.id -eq 2062})
    repeatedEntryObserved = ($reentryRuns.Count -gt 1)
    allReentryRunsOwnedDeathObserved = ($reentryRuns.Count -gt 0 -and @($reentryRuns | Where-Object {$_.ownedDeaths -eq 0}).Count -eq 0)
    allReentryReturnsCommitted = ($reentryRuns.Count -gt 0 -and @($reentryRuns | Where-Object {!$_.returnCommitted}).Count -eq 0)
    battleVerified = $false
}
[IO.File]::WriteAllText((Join-Path $destination 'summary.json'), ($summary | ConvertTo-Json -Depth 20), $utf8)
Write-Host ('采集完成：{0}' -f $destination)
Write-Host ('进度：{0}；CMD1811={1}；原生记录={2}；缺口={3}' -f $progress, $requests.Count, $native.Count, $issues.Count)
foreach ($issue in $issues) { Write-Host ('  ' + $issue) }
Write-Host '日志均留在本机；采集结果不代表战斗验收，不需要重新操作保存来生成汇总。'
exit 0

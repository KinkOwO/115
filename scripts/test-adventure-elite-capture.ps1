<#
.SYNOPSIS
  使用隔离日志夹具验证精锐采集与只读预检；不启动游戏、不修改玩家存档。
.NOTES
  退出码：0 全部通过；1 夹具或断言失败。只写 .tmp/adventure-elite/capture-test/ 自有夹具。
#>
[CmdletBinding()]
param()
$ErrorActionPreference = 'Stop'
$taskRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..')).Path
Set-Location -LiteralPath $taskRoot
$fixtureRoot = Join-Path (Join-Path $taskRoot '.tmp/adventure-elite/capture-test') ([Guid]::NewGuid().ToString('N'))
$collector = Join-Path $PSScriptRoot 'collect-adventure-elite.ps1'
$utf8 = New-Object Text.UTF8Encoding($false)
function Save-Json([string]$path, $object) { [IO.File]::WriteAllText($path, ($object | ConvertTo-Json -Depth 20), $utf8) }
function Assert($value, [string]$message) { if (!$value) { throw $message } }
foreach ($case in @('success', 'released', 'no-request', 'old-version', 'incomplete', 'truncated', 'lifecycle-success', 'lifecycle-released', 'lifecycle-inconsistent', 'lifecycle-wrong-pid', 'lifecycle-missing', 'lifecycle-post-return', 'lifecycle-unpaired-ui', 'owner-success', 'owner-missing-fields', 'owner-invalid-identity', 'owner-unbound', 'owner-duplicate', 'owner-inconsistent', 'owner-scene', 'owner-wrong-player', 'entry-success', 'entry-missing', 'entry-wrong-pid', 'entry-truncated', 'entry-incomplete', 'entry-fields','registration-success','registration-rejected','registration-no-channel','registration-no-result','registration-fields','registration-invalid-scope','registration-order','registration-current','combat-success','combat-missing','combat-wrong-pid','combat-truncated','combat-incomplete','combat-fields','combat-unpaired','combat-death-mismatch','combat-owner-mismatch','owned-success','owned-invalid-rewrite','owned-no-rewrite','owned-wrong-effective','owned-unpaired','owned-foreign-owner','owned-no-server','owned-stale-run','owned-victim-kind','owned-other-room','owned-other-source','owned-reentry-success','owned-reentry-wrong-run','owned-reentry-wrong-serial','owned-reentry-no-return','owned-reentry-missing-channel','owned-reentry-stale-start','owned-reentry-no-entry-commit','owned-reentry-adapter-success','owned-reentry-adapter-missing','owned-reentry-adapter-room','owned-reentry-adapter-order','owned-reentry-adapter-write','owned-reentry-adapter-result','owned-reentry-adapter-member','owned-reentry-adapter-fields','owned-reentry-adapter-count','owned-reentry-adapter-retry-success','owned-reentry-adapter-retry-no-request','owned-reentry-adapter-retry-dirty','owned-reentry-adapter-retry-same-run','owned-reentry-adapter-retry-no-plan','owned-reentry-adapter-retry-duplicate','owned-reentry-adapter-retry-missing-loader')) {
    $base = Join-Path $fixtureRoot $case
    $session = Join-Path $base 'session'
    $patch = Join-Path $base 'patch'
    $out = Join-Path $base 'out'
    New-Item -ItemType Directory -Path $session, $patch -Force | Out-Null
    [IO.File]::WriteAllText((Join-Path $patch 'AdventureElite.dll'), 'offline-collector-fixture', $utf8)
    $artifacts = @()
    foreach ($rel in @('client-patchs/adventure-elite/dist/AdventureElite.dll', 'server/work/dfo-lan/bin/wireprobe-handoff-source.exe', 'server/work/dfo-lan/bin/dfolauncher-adventure-elite.exe')) {
        $path = if ($rel.EndsWith('AdventureElite.dll')) { Join-Path $patch 'AdventureElite.dll' } else { Join-Path $taskRoot $rel }
        $artifacts += @{ path = $rel; sha256 = (Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash.ToLowerInvariant() }
    }
    $version = if ($case.StartsWith('owned-reentry-adapter-')) { '0.3.9' } elseif ($case.StartsWith('owned-')) { '0.3.8' } elseif (($case.StartsWith('combat-') -or $case.StartsWith('owned-'))) { '0.3.7' } elseif ($case -eq 'registration-current') { '0.3.6' } elseif (($case.StartsWith('registration-') -or ($case.StartsWith('combat-') -or $case.StartsWith('owned-')))) { '0.3.5' } elseif ($case.StartsWith('entry-')) { '0.3.4' } elseif ($case.StartsWith('owner-')) { '0.3.3' } elseif ($case.StartsWith('lifecycle-')) { '0.3.2' } else { '0.3.1' }
    Save-Json (Join-Path $patch 'adventure-elite-build.json') @{ version = $version; artifacts = $artifacts }
    $traceName = 'adventure-elite-native-66600001-8000.jsonl'
    $status = @{ version = $version; processId = 66600001; runTick = 8000; nativeTraceFile = $traceName; ordinaryPrepareReady = $true }
    if ($case -eq 'old-version') { $status.version = '0.2.2'; $status.processId = 66600002 }
    if ($case.StartsWith('lifecycle-') -or $case.StartsWith('owner-') -or $case.StartsWith('entry-') -or ($case.StartsWith('registration-') -or ($case.StartsWith('combat-') -or $case.StartsWith('owned-')))) {
        $status.ordinaryLifecycleReady = $true
        $status.lifecycleTraceFile = 'adventure-elite-lifecycle-66600001-8000.jsonl'
        if ($case -ne 'lifecycle-missing') {
            $life = @(
                @{ processId = 66600001; runTick = 8000; ordinal = 1; tick = 30; cause = 'native-load-completed'; available = $true; consistent = $true; identityMatches = $true; weakAlive = 1; traceDropped = 0 },
                @{ processId = 66600001; runTick = 8000; ordinal = 2; tick = 40; cause = 'native-flush'; available = $true; consistent = ($case -ne 'lifecycle-inconsistent'); identityMatches = $true; weakAlive = $(if ($case -eq 'lifecycle-released') { 0 } else { 1 }); traceDropped = 0 }
            )
            if ($case -eq 'lifecycle-wrong-pid') { $life[1].processId = 66600002 }
            [IO.File]::WriteAllText((Join-Path $patch $status.lifecycleTraceFile), (($life | ForEach-Object { $_ | ConvertTo-Json -Compress }) -join [Environment]::NewLine), $utf8)
        }
    }
    if ($case.StartsWith('owner-') -or $case.StartsWith('entry-') -or ($case.StartsWith('registration-') -or ($case.StartsWith('combat-') -or $case.StartsWith('owned-')))) {
        $status.lifecycleTraceVersion = 2
        foreach ($row in $life) {
            $row.traceVersion = 2; $row.currentOwner = 2
            $row.weakStrong = @(1,0,0); $row.identityWords = @(@(522846793,522847006),@(0,0),@(0,0))
            $row.identityValid = @(1,-1,-1); $row.objectId = @(17,-1,-1)
            $row.ownerStrong = @(1,-1,-1); $row.ownerIdentityValid = @(1,-1,-1)
            $row.ownerObjectId = @(2,-1,-1); $row.ownerControllerId = @(2,-1,-1)
            $row.ownerMatchesCurrentPlayer = @(1,-1,-1); $row.ownerKind = @(3,-1,-1); $row.sceneAttached = @(0,-1,-1)
            if ($case -eq 'owner-missing-fields') { $row.Remove('objectId') }
            if ($case -eq 'owner-invalid-identity') { $row.identityValid[0] = 0 }
            if ($case -eq 'owner-wrong-player') { $row.ownerMatchesCurrentPlayer[0] = 0 }
            if ($case -eq 'owner-unbound') { $row.ownerControllerId[0] = 7 }
            if ($case -eq 'owner-duplicate') {
                $row.weakStrong[1] = 1; $row.identityValid[1] = 1; $row.objectId[1] = 17
                $row.ownerStrong[1] = 1; $row.ownerIdentityValid[1] = 1; $row.ownerObjectId[1] = 2; $row.ownerControllerId[1] = 2
            }
            if ($case -eq 'owner-inconsistent') { $row.consistent = $false }
            if ($case -eq 'owner-scene') { $row.sceneAttached[0] = 1 }
        }
        [IO.File]::WriteAllText((Join-Path $patch $status.lifecycleTraceFile), (($life | ForEach-Object { $_ | ConvertTo-Json -Compress -Depth 5 }) -join [Environment]::NewLine), $utf8)
    }
    Save-Json (Join-Path $patch 'adventure-elite.status.json') $status
    [IO.File]::WriteAllText((Join-Path $session 'client.log'), "10 ROOT_PID 66600001" + [Environment]::NewLine + '50 SUMMARY exit=0x0', $utf8)
    foreach ($name in @('helper.err', 'helper.out', 'run.json')) { [IO.File]::WriteAllText((Join-Path $session $name), '{}', $utf8) }
    [IO.File]::WriteAllText((Join-Path $patch 'adventure-elite.log'), 'offline fixture', $utf8)
    $events = @(@{ kind = 'client_frame'; id = 15 }, @{ kind = 'client_frame'; id = 132 }, @{ kind = 'client_frame'; id = 1719 }, @{ kind = '精锐角色设置同步'; id = 1754 })
    if ($case -ne 'no-request') { $events += @{ kind = 'client_frame'; id = 1811 }; $events += @{ kind = 'adventure_elite_transaction_decision'; id = 1811; accepted = $true } }
    if ($case -in @('lifecycle-post-return', 'lifecycle-unpaired-ui')) {
        $events += @{ kind = 'adventure_elite_packet_sent'; id = 1879 }
        $events += @{ kind = 'dungeon_selection_return' }
        $events += @{ kind = 'client_frame'; id = 1395 }
        if ($case -eq 'lifecycle-unpaired-ui') { $events += @{ kind = 'client_frame'; id = 1395 } }
        $life[1].callerRva = 0x3C768AF
        $life[1].weakStrong = @(1,0,0); $life[1].actorKind = @(5,-1,-1)
        $life[1].controllerBound = @(1,-1,-1); $life[1].controllerId = @(7,-1,-1); $life[1].controllerWire = @(7,-1,-1)
        [IO.File]::WriteAllText((Join-Path $patch $status.lifecycleTraceFile), (($life | ForEach-Object { $_ | ConvertTo-Json -Compress }) -join [Environment]::NewLine), $utf8)
    }
    if ($case.StartsWith('entry-') -or ($case.StartsWith('registration-') -or ($case.StartsWith('combat-') -or $case.StartsWith('owned-')))) {
        $status.ordinaryEntryTraceReady=$true; $status.entryTraceFile='adventure-elite-entry-66600001-8000.jsonl'
        Save-Json (Join-Path $patch 'adventure-elite.status.json') $status
        $events += @{kind='adventure_elite_entry_probe'; accepted=$true; battle_enabled=$false}
        $entry=@()
        foreach ($phase in @('loader-before','registration-gate','loader-after')) {
            if ($case -eq 'entry-incomplete' -and $phase -eq 'loader-after') { continue }
            $entry += @{traceVersion=1;processId=66600001;runTick=8000;phase=$phase;callerRva=0x5B25191;traceDropped=0;traceLimit=0;battleEnabled=$false;objectId=@(0,1,-1);ownerMatchesCurrentPlayer=@(1,1,-1);controllerId=@(65535,65535,-1);vtable=@(1,1,0);ownerGetter=@(1,1,0);combatGetter=@(1,1,0);sceneVectorMatches=@(0,0,-1)}
        }
        if (($case.StartsWith('registration-') -or ($case.StartsWith('combat-') -or $case.StartsWith('owned-')))) {
            $status.ordinaryRegistrationReady=$true
            Save-Json (Join-Path $patch 'adventure-elite.status.json') $status
            $phases=@('registration-scope-allowed','registration-channel-native','native-registration-result')
            if ($case -eq 'registration-rejected') { $phases=@('registration-scope-rejected') }
            $entry += @($phases | ForEach-Object { @{traceVersion=1;processId=66600001;runTick=8000;phase=$_;callerRva=$(if($_ -eq 'registration-channel-native'){0x5B251C3}elseif($_ -eq 'native-registration-result'){0x5B2529D}else{0x5B251AE});traceDropped=0;traceLimit=0;battleEnabled=$false;nativeResult=1;objectId=@(0,1,-1);ownerMatchesCurrentPlayer=@(1,1,-1);controllerId=@(65535,65535,-1);vtable=@(1,1,0);ownerGetter=@(1,1,0);combatGetter=@(1,1,0);sceneVectorMatches=@(1,1,-1)} })
            $after=@($entry | Where-Object {$_.phase -eq 'loader-after'})
            $entry=@($entry | Where-Object {$_.phase -ne 'loader-after'}) + $after
            $ordinal=0
            foreach($row in $entry) { $row.ordinal=++$ordinal; $row.thread=123; $row.registrationChecks=@(1,1,1,1,1,1,1,1,1,1,2); $row.registrationPlayerRef=@(100,200); $row.registrationNativePlayerRef=@(100,200) }
            if($case -eq 'registration-rejected') { $entry[-1].nativeResult=0; $entry[-1].registrationChecks[6]=0; $entry[-1].registrationNativePlayerRef=@(300,400) }
            if($case -eq 'registration-no-channel') { $entry=@($entry | Where-Object {$_.phase -ne 'registration-channel-native'}) }
            if($case -eq 'registration-no-result') { $entry=@($entry | Where-Object {$_.phase -ne 'native-registration-result'}) }
            if($case -eq 'registration-order') { ($entry | Where-Object {$_.phase -eq 'loader-after'}).ordinal=2 }
            if($case -eq 'registration-fields') { $entry[1].Remove('registrationChecks') }
            if($case -eq 'registration-invalid-scope') { ($entry | Where-Object {$_.phase -eq 'registration-scope-allowed'}).registrationChecks[6]=0 }
        }
        if ($case -eq 'entry-wrong-pid') { $entry[1].processId=66600002 }
        if ($case -eq 'entry-truncated') { $entry[1].traceLimit=1 }
        if ($case -eq 'entry-fields') { $entry[1].Remove('vtable') }
        if ($case -ne 'entry-missing') {
            [IO.File]::WriteAllText((Join-Path $patch $status.entryTraceFile), (($entry | ForEach-Object {$_ | ConvertTo-Json -Compress}) -join [Environment]::NewLine),$utf8)
        }
    }
    if (($case.StartsWith('combat-') -or $case.StartsWith('owned-'))) {
        $status.ordinaryCombatTraceReady=$true; $status.combatTraceVersion=1; $status.combatTraceFile='adventure-elite-combat-66600001-8000.jsonl'
        Save-Json (Join-Path $patch 'adventure-elite.status.json') $status
        $combat=@()
        foreach($phase in @('killer-write','source-assignment','death-send')) {
            $ordinal=$combat.Count+1
            $combat+=@{traceVersion=1;processId=66600001;runTick=8000;ordinal=$ordinal;tick=100+$ordinal;thread=9;phase=$phase;transaction=$(if($ordinal -eq 1){2}elseif($ordinal -eq 2){1}else{3});parent=$(if($ordinal -eq 1){1}else{0});callerRva=$(if($ordinal -eq 1){0x5DD022B}else{0});input=77;argument=65535;completed=1;result=0;traceDropped=0;traceLimit=0;victim=1234;victimId=4096;victimKind=4;readable=@(1,1);stable=@(1,1);killer=@(65535,65535);sourceField=@(0,0);sourceRef=@(1,2,3);sourceId=0;sourceKind=5;sourceStrong=1;sourceController=65535;combatGetter=1;eliteSlot=0;ownerId=2;ownerController=2;ownerMatches=1;playerRef=@(2,3);battleEnabled=$false;nativeWritesChanged=$false}
        }
        if($case -eq 'combat-wrong-pid'){$combat[1].processId=3}
        if($case -eq 'combat-truncated'){$combat[2].traceLimit=1}
        if($case -eq 'combat-incomplete'){$combat[2].completed=0}
        if($case -eq 'combat-fields'){$combat[1].Remove('sourceRef')}
        if($case -eq 'combat-unpaired'){$combat[1].input=78}
        if($case -eq 'combat-owner-mismatch'){$combat[0].ownerMatches=0}
        if($case -ne 'combat-missing'){
            [IO.File]::WriteAllText((Join-Path $patch $status.combatTraceFile),(($combat|ForEach-Object{$_|ConvertTo-Json -Compress -Depth 6}) -join [Environment]::NewLine),$utf8)
        }
        $events+=@{kind='client_frame';id=39;plain_hex=$(if($case -eq 'combat-death-mismatch'){'01100000ffff'+'00'*58}else{'00100000ffff'+'00'*58})}
    }
    if($case.StartsWith('owned-')) {
        $status.combatTraceVersion=2; Save-Json (Join-Path $patch 'adventure-elite.status.json') $status
        foreach($r in $combat){$r.victimKind=3;$r.traceVersion=2;$r.effectiveArgument=$r.argument;$r.uniqueSource=1;$r.controllerBound=1;$r.controllerWire=65535;$r.combatGetterRva=0x14CC20;$r.roomSerial=1}
        $combat[0].nativeWritesChanged=$true;$combat[0].effectiveArgument=2;$combat[0].killer[1]=2
        $combat[1].killer[1]=2;$combat[2].killer=@(2,2)
        $events=@($events|Where-Object{$_.id -ne 39})
        $events+=@{kind='client_frame';id=39;plain_hex='001000000200'+'00'*58}
        if($case -eq 'owned-victim-kind'){$combat[0].victimKind=4}
        if($case -eq 'owned-other-room'){$combat[2].roomSerial=2}
        if($case -eq 'owned-other-source'){$combat[2].sourceRef=@(4,5,6)}
        if($case -eq 'owned-invalid-rewrite'){$combat[0].callerRva=0}
        if($case -eq 'owned-no-rewrite'){$combat[0].nativeWritesChanged=$false}
        if($case -eq 'owned-wrong-effective'){$combat[0].effectiveArgument=7}
        if($case -eq 'owned-unpaired'){$combat[0].parent=0}
        if($case -eq 'owned-foreign-owner'){$combat[0].ownerId=7}
        [IO.File]::WriteAllText((Join-Path $patch $status.combatTraceFile),(($combat|ForEach-Object{$_|ConvertTo-Json -Compress -Depth 6}) -join [Environment]::NewLine),$utf8)
        if($case -ne 'owned-no-server'){
            $events+=@{kind='adventure_elite_combat_request';candidate_version='0.3.8';id=39;processed=$true;owner_wire_id=2;character_id=2;channel_type=22;frozen_owner=$(if($case -eq 'owned-stale-run'){7}else{2});frozen_channel=22;client_acceptance='pending';before=@{dead=$false};after=@{entity=4096;killer=2;dead=$true;unowned=$false}}
        }
    }
    if($case.StartsWith('owned-reentry-')) {
        $events=@($events | Where-Object {$_.kind -notin @('adventure_elite_entry_probe','adventure_elite_combat_request') -and $_.id -ne 39})
        $templateEntry=$entry; $templateCombat=$combat; $entry=@(); $combat=@()
        for($generation=1;$generation -le 2;$generation++) {
            $runId='run-'+$generation
            $events+=@{kind='adventure_elite_entry_probe';accepted=$true;battle_enabled=$false;candidate_stage='ordinary-reentry';run_id=$(if($case -eq 'owned-reentry-wrong-run'){'run-1'}else{$runId});entry_serial=$generation;dungeon=3}
            if($case -ne 'owned-reentry-no-entry-commit' -or $generation -eq 1){$events+=@{kind='adventure_elite_dispatch_committed';id=16;entry_serial=$generation;after=@{run_id=$runId;death_sent_count=$(if($case -eq 'owned-reentry-stale-start' -and $generation -eq 2){1}else{0});drops_active=$false;cards_active=$false;result_sent=$false;completion_sent=$false}}}
            foreach($r in $templateEntry){$copy=$r.Clone();$copy.ordinal=$r.ordinal+($generation-1)*$templateEntry.Count;$entry+=$copy}
            foreach($r in $templateCombat){$copy=$r.Clone();$copy.ordinal=$r.ordinal+($generation-1)*3;$copy.transaction=$r.transaction+($generation-1)*3;if($r.parent){$copy.parent=$r.parent+($generation-1)*3};$copy.roomSerial=$generation;$combat+=$copy}
            $events+=@{kind='client_frame';id=39;plain_hex='001000000200'+'00'*58}
            $events+=@{kind='adventure_elite_combat_request';candidate_version='0.3.8';candidate_stage='ordinary-reentry';entry_serial=$generation;id=39;processed=$true;owner_wire_id=2;character_id=2;channel_type=22;frozen_owner=2;frozen_channel=22;client_acceptance='pending';before=@{dead=$false;run_id=$runId;entry_serial=$(if($generation -eq 2 -and $case -eq 'owned-reentry-wrong-serial'){1}else{$generation})};after=@{entity=4096;killer=2;dead=$true;unowned=$false}}
            if($case -ne 'owned-reentry-no-return' -or $generation -eq 1){$events+=@{kind='adventure_elite_dispatch_committed';id=72;origin_run_id=$runId;entry_serial=$generation;after=@{active=$false;pending_town_arrival=$false;death_sent_count=0;drops_active=$false;cards_active=$false;result_sent=$false;completion_sent=$false}}}
        }
        if($case -eq 'owned-reentry-missing-channel'){$entry=@($entry | Where-Object {$_.phase -ne 'registration-channel-native' -or $_.ordinal -gt $templateEntry.Count})}
        [IO.File]::WriteAllText((Join-Path $patch $status.entryTraceFile),(($entry|ForEach-Object{$_|ConvertTo-Json -Compress -Depth 6}) -join [Environment]::NewLine),$utf8)
        [IO.File]::WriteAllText((Join-Path $patch $status.combatTraceFile),(($combat|ForEach-Object{$_|ConvertTo-Json -Compress -Depth 6}) -join [Environment]::NewLine),$utf8)
    }
    if($case.StartsWith('owned-reentry-adapter-')) {
        $newEntry=@();$ordinal=0;$generation=0
        foreach($r in $entry) {
            $row=$r.Clone();$row.registrationChecks=$r.registrationChecks.Clone();$row.sceneVectorMatches=$r.sceneVectorMatches.Clone()
            if($row.phase -eq 'loader-before'){$generation++}
            $row.loaderCallerRva=0x6D2A1E7;$row.freshDungeonEntry=$true;$row.weakAlive=2;$row.manager576=$(if($generation -eq 1){0}else{1})
            if($row.phase -in @('loader-before','loader-after')){$row.callerRva=0x6D2A1E7}
            if($row.phase -eq 'loader-after'){$row.sceneVectorMatches=@(1,1,-1)}
            $row.ordinal=++$ordinal;$newEntry+=$row
            if($generation -eq 2 -and $row.phase -eq 'registration-channel-native') {
                $active=$row.Clone();$active.phase='registration-active-reentry';$active.callerRva=0x5B251D8;$active.nativeResult=1;$active.ordinal=++$ordinal;$newEntry+=$active
            }
            if($row.phase -eq 'native-registration-result'){$second=$row.Clone();$second.ordinal=++$ordinal;$newEntry+=$second}
        }
        $entry=$newEntry
        $active=@($entry | Where-Object {$_.phase -eq 'registration-active-reentry'})[0]
        if($case -eq 'owned-reentry-adapter-missing'){$entry=@($entry | Where-Object {$_.phase -ne 'registration-active-reentry'})}
        if($case -eq 'owned-reentry-adapter-room'){$active.loaderCallerRva=0x6CE14C1;$active.freshDungeonEntry=$false}
        if($case -eq 'owned-reentry-adapter-order'){$active.ordinal=$entry[-1].ordinal+1}
        if($case -eq 'owned-reentry-adapter-write'){$active.manager576=0}
        if($case -eq 'owned-reentry-adapter-result'){@($entry | Where-Object {$_.phase -eq 'native-registration-result'})[-1].nativeResult=0}
        if($case -eq 'owned-reentry-adapter-member'){$entry[-1].sceneVectorMatches[1]=0}
        if($case -eq 'owned-reentry-adapter-fields'){$entry[-1].Remove('loaderCallerRva')}
        if($case -eq 'owned-reentry-adapter-count'){$entry=@($entry | Where-Object {$_.ordinal -ne 13})}
        [IO.File]::WriteAllText((Join-Path $patch $status.entryTraceFile),(($entry|ForEach-Object{$_|ConvertTo-Json -Compress -Depth 6}) -join [Environment]::NewLine),$utf8)
    }
    if($case.StartsWith('owned-reentry-adapter-retry-')) {
        $events=@($events | Where-Object {!($_.kind -eq 'adventure_elite_entry_probe' -and $_.entry_serial -eq 2) -and !($_.kind -eq 'adventure_elite_dispatch_committed' -and $_.id -eq 16 -and $_.entry_serial -eq 2)})
        foreach($r in $events){if($r.entry_serial -eq 2){$r.entry_serial=1;if($r.before){$r.before.entry_serial=1}}}
        $request=@{kind='adventure_elite_combat_request';candidate_version='0.3.8';id=72;processed=$true;owner_wire_id=2;client_acceptance='pending';entry_serial=1;frozen_owner=2;character_id=2;frozen_channel=22;channel_type=22;before=@{run_id='run-1';entry_serial=1;completed=$true;result_sent=$true};packets=@(@{id=16;kind=1})}
        $transition=@{kind='adventure_elite_dispatch_committed';id=72;origin_run_id='run-1';entry_serial=1;after=@{run_id='run-2';active=$true;loaded=$false;completed=$false;death_sent_count=0;drops_active=$false;cards_active=$false;result_sent=$false;completion_sent=$false}}
        if($case.EndsWith('-dirty')){$transition.after.death_sent_count=1}
        if($case.EndsWith('-same-run')){$transition.after.run_id='run-1'}
        if($case.EndsWith('-no-plan')){$request.packets=@()}
        if(!$case.EndsWith('-no-request')){$events+=$request}
        $events+=$transition
        if($case.EndsWith('-duplicate')){$events+=$transition.Clone()}
        if($case.EndsWith('-missing-loader')){$entry=@($entry | Where-Object {$_.ordinal -lt 8});[IO.File]::WriteAllText((Join-Path $patch $status.entryTraceFile),(($entry|ForEach-Object{$_|ConvertTo-Json -Compress -Depth 6}) -join [Environment]::NewLine),$utf8)}
    }
    [IO.File]::WriteAllText((Join-Path $session 'events.jsonl'), (($events | ForEach-Object { $_ | ConvertTo-Json -Compress }) -join [Environment]::NewLine), $utf8)
    $rows = @()
    $sequence = 0
    foreach ($opcode in @(1754, 1382, 1879)) {
        if ($case -eq 'no-request' -and $opcode -ne 1754) { continue }
        $sequence++
        foreach ($phase in @('arrival', 'before', 'after', 'scope-closed')) {
            if ($case -eq 'incomplete' -and $opcode -eq 1879 -and $phase -eq 'scope-closed') { continue }
            $rows += @{ processId = 66600001; runTick = 8000; tick = $sequence*10; sequence = $sequence; opcode = $opcode; phase = $phase; readerSuccess = 1; weakAlive = $(if ($opcode -eq 1879) { 1 } else { 0 }); traceDropped = 0; scopeReason = $(if ($case -eq 'no-request') { 'identity-unavailable' } else { 'transaction-matched' }) }
        }
    }
    if ($case -eq 'released') {
        foreach ($phase in @('arrival', 'before', 'after', 'scope-closed')) {
            $rows += @{ processId = 66600001; runTick = 8000; sequence = 4; opcode = 1754; phase = $phase; readerSuccess = 1; weakAlive = $(if ($phase -in @('arrival', 'before')) { 1 } else { 0 }); traceDropped = 0; scopeReason = 'selection-nonempty' }
        }
    }
    if ($case -eq 'truncated') { $rows += @{ processId = 66600001; runTick = 8000; sequence = 5; opcode = 1754; phase = 'trace-limit'; traceDropped = 1; scopeReason = 'selection-nonempty' } }
    [IO.File]::WriteAllText((Join-Path $patch $traceName), (($rows | ForEach-Object { $_ | ConvertTo-Json -Compress }) -join [Environment]::NewLine), $utf8)
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $collector -CheckOnly -PatchDirectory $patch -OutputRoot $out | Out-Null
    Assert ($LASTEXITCODE -eq 0) ($case + ': preflight failed')
    Assert (!(Test-Path -LiteralPath $out)) ($case + ': read-only preflight wrote output')
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $collector -SessionPath $session -PatchDirectory $patch -OutputRoot $out | Out-Null
    Assert ($LASTEXITCODE -eq 0) ($case + ': capture failed')
    $summary = [IO.File]::ReadAllText((Join-Path $out 'session/summary.json')) | ConvertFrom-Json
    Assert (!$summary.battleVerified) 'town logs cannot certify battle'
    if ($case -eq 'success') { Assert $summary.coverageComplete 'valid run lost coverage'; Assert ($summary.progress -eq 'client-clone-observed-battle-unverified') 'clone observation missing' }
    elseif ($case -eq 'released') { Assert $summary.coverageComplete 'release evidence lost coverage'; Assert ($summary.progress -eq 'client-clone-observed-then-released') 'later release must supersede initial clone success'; Assert $summary.cloneReleasedAfterCompletion 'release flag missing'; Assert (!$summary.cloneAliveAtLastObservation) 'released companions cannot be reported alive' }
    elseif ($case -eq 'no-request') { Assert ($summary.progress -eq 'no-native-load-request') 'missing request must be explicit'; Assert ($summary.nativeReasons[0].Name -eq 'identity-unavailable') 'identity rejection lost' }
    elseif ($case.StartsWith('owned-reentry-adapter-retry-')) {
        Assert ($summary.allFreshEntriesRegistered -eq $case.EndsWith('-success')) ($case+': retry registration false positive')
        if($case.EndsWith('-success')){Assert ($summary.freshServerEntryCount -eq 2 -and $summary.retryRuns.Count -eq 1 -and $summary.retryRuns[0].ownedDeaths -eq 1) 'retry must retain committed run and owned death without synthesizing server serial'}
        else{Assert ($summary.issues.Count -gt 0) ($case+': retry evidence gap missing')}
    }
    elseif ($case.StartsWith('owned-reentry-adapter-')) {
        Assert ($summary.allFreshEntriesRegistered -eq ($case -eq 'owned-reentry-adapter-success')) ($case+': fresh registration false positive')
        if($case -eq 'owned-reentry-adapter-success'){Assert $summary.allReentryRunsOwnedDeathObserved 'v0.3.9 owned deaths missing';Assert ($summary.registrationActiveReentryObservations.Count -eq 1) 'active consumer missing'}else{Assert ($summary.issues.Count -gt 0) 'invalid reentry evidence not rejected'}
    }
    elseif ($case.StartsWith('owned-reentry-')) {
        Assert $summary.repeatedEntryObserved 'multiple run decisions missing'
        Assert ($summary.allReentryReturnsCommitted -eq ($case -ne 'owned-reentry-no-return')) 'committed return coverage incorrect'
        Assert ($summary.entryProbeCoverageComplete -eq ($case -in @('owned-reentry-success','owned-reentry-no-return'))) 'repeat loader or run identity coverage incorrect'
        if($case -eq 'owned-reentry-success'){Assert $summary.allReentryRunsOwnedDeathObserved 'missing per-run owned deaths'; Assert ($summary.ownedCombatFreshDeaths.Count -eq 2) 'reused entity ID paired across wrong generation'}
        if($case -in @('owned-reentry-wrong-run','owned-reentry-wrong-serial','owned-reentry-missing-channel','owned-reentry-stale-start','owned-reentry-no-entry-commit')){Assert ($summary.issues.Count -gt 0) 'reentry evidence defect missing'}
    }
    elseif ($case.StartsWith('owned-')) { Assert ($summary.ownedCombatProcessedObserved -eq ($case -eq 'owned-success')) ($case+': owned combat false positive'); if($case -ne 'owned-success'){Assert ($summary.issues.Count -gt 0) ($case+': owned gap missing')} }
    elseif ($case.StartsWith('combat-')) { Assert ($summary.combatCoverageComplete -eq ($case -eq 'combat-success')) ($case+': combat coverage invalid'); if($case -ne 'combat-success'){Assert ($summary.combatIssues.Count -gt 0) ($case+': combat gap missing')} }
    elseif ($case.StartsWith('registration-')) { Assert ($summary.entryProbeCoverageComplete -eq ($case -in @('registration-success','registration-current','registration-rejected'))) 'registration coverage invalid'; Assert ($summary.registrationScopeAccepted -eq ($case -ne 'registration-rejected')) 'scope decision lost'; if($case -notin @('registration-success','registration-current','registration-rejected')) {Assert ($summary.issues.Count -gt 0) 'registration gap missing'} }
    elseif ($case.StartsWith('entry-')) { Assert ($summary.entryProbeCoverageComplete -eq ($case -eq 'entry-success')) 'entry coverage invalid'; if($case -ne 'entry-success') { Assert ($summary.issues.Count -gt 0) 'missing entry issue' } }
    elseif ($case.StartsWith('owner-')) {
        Assert ($summary.coverageComplete -eq ($case -ne 'owner-missing-fields')) 'owner schema coverage invalid'
        Assert (($summary.ownerIdentitySamples -gt 0) -eq ($case -in @('owner-success', 'owner-scene'))) 'owner equality/identity/uniqueness/race gate invalid'
        Assert ($summary.sceneAttachedObserved -eq ($case -eq 'owner-scene')) 'scene observation invalid'
    }
    elseif ($case.StartsWith('lifecycle-')) {
        Assert $summary.selectionReturnObserved 'selection request/cancel path lost'
        Assert ($summary.postReturnCloneVerified -eq ($case -eq 'lifecycle-post-return')) 'post-return requires accepted cancel, exact caller/count correlation and consistent type5 bindings'
        if ($case -in @('lifecycle-missing', 'lifecycle-wrong-pid')) { Assert (!$summary.coverageComplete) 'bad lifecycle capture falsely complete' }
        else {
            Assert $summary.coverageComplete 'valid lifecycle coverage lost'
            if ($case -eq 'lifecycle-released') { Assert $summary.cloneReleasedAfterCompletion 'release not detected'; Assert (!$summary.cloneAliveAtLastObservation) 'lifecycle release falsely alive' }
            else { Assert (!$summary.cloneReleasedAfterCompletion) 'inconsistent observation fabricated release' }
        }
    }
    else { Assert (!$summary.coverageComplete) ($case + ': bad/partial run falsely complete'); Assert ($summary.issues.Count -gt 0) ($case + ': missing issue') }
}
Write-Host '采集离线测试通过：78种夹具（含战斗来源父子调用、死亡向量、错进程、截断、异常、缺字段与主人不一致），含生命周期丢失/释放/不一致/错进程、选图返回配对与不配对、身份/主人/场景采样、只读预检；未启动游戏。'

# Reuse the complete native reentry fixture with Odyssey-labelled server rows.
# A server stage/version must not certify native acceptance or suppress gaps.
$odysseyBase=Join-Path $fixtureRoot 'odyssey-source-reentry'
New-Item -ItemType Directory -Path $odysseyBase | Out-Null
$reference=Join-Path $fixtureRoot 'owned-reentry-adapter-success'
Copy-Item -LiteralPath (Join-Path $reference 'session'),(Join-Path $reference 'patch') -Destination $odysseyBase -Recurse
$odysseySession=Join-Path $odysseyBase 'session'
$odysseyPatch=Join-Path $odysseyBase 'patch'
$odysseyOutput=Join-Path $odysseyBase 'out'
$odysseyEvents=@([IO.File]::ReadAllLines((Join-Path $odysseySession 'events.jsonl')) | ForEach-Object {$_ | ConvertFrom-Json})
foreach($row in $odysseyEvents) {
 if($row.kind -eq 'adventure_elite_entry_probe') {
  $row | Add-Member -NotePropertyName odyssey -NotePropertyValue $true -Force
  $row | Add-Member -NotePropertyName requested_source_odyssey -NotePropertyValue $true -Force
 }
 if($row.kind -match '^adventure_elite_(entry_probe|combat_request|dispatch_committed)$') {
  $row | Add-Member -NotePropertyName candidate_stage -NotePropertyValue 'ordinary-odyssey' -Force
 }
 if($row.kind -eq 'adventure_elite_combat_request') {$row.candidate_version='0.3.12'}
}
$odysseyEvents+= [pscustomobject]@{kind='packet_sent';id=2856;name='odyssey_journal_updated_on_return'}
[IO.File]::WriteAllText((Join-Path $odysseySession 'events.jsonl'),(($odysseyEvents | ForEach-Object {$_ | ConvertTo-Json -Depth 20 -Compress}) -join [Environment]::NewLine),$utf8)
& powershell.exe -NoProfile -ExecutionPolicy Bypass -File $collector -SessionPath $odysseySession -PatchDirectory $odysseyPatch -OutputRoot $odysseyOutput | Out-Null
Assert ($LASTEXITCODE -eq 0) 'Odyssey observation capture failed'
$odysseySummary=[IO.File]::ReadAllText((Join-Path $odysseyOutput 'session/summary.json')) | ConvertFrom-Json
Assert ($odysseySummary.odysseyEntryObservations.Count -eq 2) 'Odyssey entry identity lost'
Assert ($odysseySummary.odysseyFlowObservations.Count -eq 1) 'native journal packet observation lost'
Assert ($odysseySummary.ownedCombatIssues.Count -eq 0) 'server0.3.12 candidate falsely rejected'
Assert $odysseySummary.allFreshEntriesRegistered 'Odyssey stage broke fresh loader pairing'
Assert (!$odysseySummary.odysseyVerified -and !$odysseySummary.battleVerified) 'source stage fabricated live acceptance'
Write-Host 'Odyssey labelled source/reentry observation fixture passed; native acceptance remains pending.'

# Same native schema with the narrow server pet-key projection; no DLL change.
foreach($row in $odysseyEvents) {
 if($row.kind -eq 'adventure_elite_combat_request') {$row.candidate_version='0.3.14'}
}
[IO.File]::WriteAllText((Join-Path $odysseySession 'events.jsonl'),(($odysseyEvents | ForEach-Object {$_ | ConvertTo-Json -Depth 20 -Compress}) -join [Environment]::NewLine),$utf8)
& powershell.exe -NoProfile -ExecutionPolicy Bypass -File $collector -SessionPath $odysseySession -PatchDirectory $odysseyPatch -OutputRoot $odysseyOutput | Out-Null
Assert ($LASTEXITCODE -eq 0) 'pet-key server0.3.14 capture failed'
$petKeySummary=[IO.File]::ReadAllText((Join-Path $odysseyOutput 'session/summary.json')) | ConvertFrom-Json
Assert ($petKeySummary.ownedCombatIssues.Count -eq 0 -and $petKeySummary.allFreshEntriesRegistered) 'server0.3.14 rejected unchanged native schema'
Assert (!$petKeySummary.odysseyVerified -and !$petKeySummary.battleVerified) 'pet-key offline fixture fabricated live acceptance'
Write-Host 'Pet-key server0.3.14 capture fixture passed; native live acceptance remains pending.'

# Same native two-entry traces; second server entry now originates at CMD2062.
foreach($case in @('success','wrong-target','dirty','no-plan','no-request','duplicate')) {
 $base=Join-Path $fixtureRoot ('odyssey-direct-'+$case)
 New-Item -ItemType Directory -Path $base | Out-Null
 Copy-Item -LiteralPath (Join-Path $odysseyBase 'session'),(Join-Path $odysseyBase 'patch') -Destination $base -Recurse
 $session=Join-Path $base 'session';$patch=Join-Path $base 'patch';$output=Join-Path $base 'out'
 $source=@([IO.File]::ReadAllLines((Join-Path $session 'events.jsonl')) | ForEach-Object {$_ | ConvertFrom-Json})
 $request=[pscustomobject]@{kind='adventure_elite_combat_request';candidate_version='0.3.13';candidate_stage='ordinary-odyssey';id=2062;processed=$true;owner_wire_id=2;client_acceptance='pending';entry_serial=2;frozen_owner=2;character_id=2;frozen_channel=22;channel_type=22;pending_run_id='run-2';pending_dungeon=4;before=@{run_id='run-1';entry_serial=1;completed=$true;result_sent=$true;source_odyssey=$true;requested_source_odyssey=$true;requested_dungeon=4};packets=@(@{id=15;kind=1},@{id=27;kind=0},@{id=16;kind=1},@{id=28;kind=0},@{id=29;kind=0})}
 $transition=[pscustomobject]@{kind='adventure_elite_dispatch_committed';candidate_stage='ordinary-odyssey';id=2062;origin_run_id='run-1';entry_serial=2;after=@{run_id='run-2';dungeon=4;entry_serial=2;active=$true;loaded=$false;completed=$false;death_sent_count=0;drops_active=$false;cards_active=$false;result_sent=$false;completion_sent=$false}}
 if($case -eq 'wrong-target'){$request.before.requested_dungeon=7}
 if($case -eq 'dirty'){$transition.after.death_sent_count=1}
 if($case -eq 'no-plan'){$request.packets=@()}
 $events=@()
 foreach($row in $source) {
  if($row.kind -eq 'adventure_elite_dispatch_committed' -and $row.id -eq 72 -and $row.origin_run_id -eq 'run-1'){continue}
  if($row.kind -eq 'adventure_elite_entry_probe' -and $row.entry_serial -eq 2) {
   if($case -ne 'no-request'){$events+=$request}
   $row | Add-Member -NotePropertyName entry_opcode -NotePropertyValue 2062 -Force
  }
  if($row.kind -eq 'adventure_elite_dispatch_committed' -and $row.id -eq 16 -and $row.entry_serial -eq 2) {
   $events+=$transition
   if($case -eq 'duplicate'){$events+=$transition}
  }else{$events+=$row}
 }
 [IO.File]::WriteAllText((Join-Path $session 'events.jsonl'),(($events | ForEach-Object {$_ | ConvertTo-Json -Depth 20 -Compress}) -join [Environment]::NewLine),$utf8)
 & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $collector -SessionPath $session -PatchDirectory $patch -OutputRoot $output | Out-Null
 Assert ($LASTEXITCODE -eq 0) ('direct '+$case+': capture failed')
 $s=[IO.File]::ReadAllText((Join-Path $output 'session/summary.json')) | ConvertFrom-Json
 if($case -eq 'success') {
  Assert ($s.directMoveRuns.Count -eq 1 -and $s.directMoveRuns[0].cleanNewRun -and $s.directMoveRuns[0].requestAndSourceMatched -and $s.directMoveRuns[0].entryPlanMatched) 'direct source/plan/commit missing'
  Assert $s.allFreshEntriesRegistered 'direct must match second native fresh loader'
  Assert (!$s.allReentryReturnsCommitted) 'direct must not fabricate first run town return'
 }else{Assert ($s.issues.Count -gt 0 -and !$s.allFreshEntriesRegistered) ('direct '+$case+': invalid evidence certified')}
 Assert (!$s.odysseyVerified -and !$s.battleVerified) 'offline direct stage fabricated live acceptance'
}
Write-Host 'Odyssey direct-move capture: six fixtures passed; cross-generation/source/plan/dirty/duplicate evidence checked; no live acceptance inferred.'

# Fully occupied native roster can complete without the create-actor predicate.
# DLL0.3.10 retains the existing lifecycle/entry/combat schemas.
$fullRosterPatch=$odysseyPatch
$fullRosterStatus=[IO.File]::ReadAllText((Join-Path $fullRosterPatch 'adventure-elite.status.json')) | ConvertFrom-Json
$fullRosterBuild=[IO.File]::ReadAllText((Join-Path $fullRosterPatch 'adventure-elite-build.json')) | ConvertFrom-Json
$fullRosterStatus.version='0.3.10';$fullRosterBuild.version='0.3.10'
[IO.File]::WriteAllText((Join-Path $fullRosterPatch 'adventure-elite.status.json'),($fullRosterStatus | ConvertTo-Json -Depth 20),$utf8)
[IO.File]::WriteAllText((Join-Path $fullRosterPatch 'adventure-elite-build.json'),($fullRosterBuild | ConvertTo-Json -Depth 20),$utf8)
& powershell.exe -NoProfile -ExecutionPolicy Bypass -File $collector -SessionPath $odysseySession -PatchDirectory $fullRosterPatch -OutputRoot $odysseyOutput | Out-Null
Assert ($LASTEXITCODE -eq 0) 'DLL0.3.10 full-roster capture failed'
$fullRosterSummary=[IO.File]::ReadAllText((Join-Path $odysseyOutput 'session/summary.json')) | ConvertFrom-Json
Assert ($fullRosterSummary.ownedCombatIssues.Count -eq 0 -and $fullRosterSummary.allFreshEntriesRegistered) 'DLL0.3.10 dropped existing native schema'
Assert (!$fullRosterSummary.odysseyVerified -and !$fullRosterSummary.battleVerified) 'full-roster offline fixture fabricated live acceptance'
Write-Host 'Full-roster DLL0.3.10 capture fixture passed; native live acceptance remains pending.'

# Server0.3.15 changes only town save/preparation state; retain combat schema.
$reloadRows=@([IO.File]::ReadAllLines((Join-Path $odysseySession 'events.jsonl')) | ForEach-Object {$_ | ConvertFrom-Json})
foreach($row in $reloadRows) {
 if($row.PSObject.Properties['candidate_version']){$row.candidate_version='0.3.15'}
}
[IO.File]::WriteAllText((Join-Path $odysseySession 'events.jsonl'),(($reloadRows | ForEach-Object {$_ | ConvertTo-Json -Depth 20 -Compress}) -join [Environment]::NewLine),$utf8)
& powershell.exe -NoProfile -ExecutionPolicy Bypass -File $collector -SessionPath $odysseySession -PatchDirectory $fullRosterPatch -OutputRoot $odysseyOutput | Out-Null
Assert ($LASTEXITCODE -eq 0) 'Server0.3.15 roster-reload capture failed'
$reloadSummary=[IO.File]::ReadAllText((Join-Path $odysseyOutput 'session/summary.json')) | ConvertFrom-Json
Assert ($reloadSummary.ownedCombatIssues.Count -eq 0 -and $reloadSummary.allFreshEntriesRegistered) 'Server0.3.15 changed native evidence schema'
Assert (!$reloadSummary.odysseyVerified -and !$reloadSummary.battleVerified) 'roster-reload fixture fabricated live acceptance'
Write-Host 'Server0.3.15 roster-reload capture fixture passed; manual changed-roster acceptance remains pending.'


# Projection reload preserves the existing notification/lifecycle schemas.
foreach($row in $reloadRows) {
 if($row.PSObject.Properties['candidate_version']){$row.candidate_version='0.3.16'}
}
[IO.File]::WriteAllText((Join-Path $odysseySession 'events.jsonl'),(($reloadRows | ForEach-Object {$_ | ConvertTo-Json -Depth 20 -Compress}) -join [Environment]::NewLine),$utf8)
& powershell.exe -NoProfile -ExecutionPolicy Bypass -File $collector -SessionPath $odysseySession -PatchDirectory $fullRosterPatch -OutputRoot $odysseyOutput | Out-Null
Assert ($LASTEXITCODE -eq 0) 'Server0.3.16 projection-reload capture failed'
$projectionSummary=[IO.File]::ReadAllText((Join-Path $odysseyOutput 'session/summary.json')) | ConvertFrom-Json
Assert ($projectionSummary.ownedCombatIssues.Count -eq 0 -and $projectionSummary.allFreshEntriesRegistered) 'Server0.3.16 changed native evidence schema'
Assert (!$projectionSummary.odysseyVerified -and !$projectionSummary.battleVerified) 'projection-reload fixture fabricated live acceptance'
Write-Host 'Server0.3.16 projection-reload capture fixture passed; manual warp/APC acceptance remains pending.'


# Stable town roster changes no native capture or combat layouts.
foreach($row in $reloadRows) {
 if($row.PSObject.Properties['candidate_version']){$row.candidate_version='0.3.17'}
}
[IO.File]::WriteAllText((Join-Path $odysseySession 'events.jsonl'),(($reloadRows | ForEach-Object {$_ | ConvertTo-Json -Depth 20 -Compress}) -join [Environment]::NewLine),$utf8)
& powershell.exe -NoProfile -ExecutionPolicy Bypass -File $collector -SessionPath $odysseySession -PatchDirectory $fullRosterPatch -OutputRoot $odysseyOutput | Out-Null
Assert ($LASTEXITCODE -eq 0) 'Server0.3.17 stable-warp capture failed'
$warpSummary=[IO.File]::ReadAllText((Join-Path $odysseyOutput 'session/summary.json')) | ConvertFrom-Json
Assert ($warpSummary.ownedCombatIssues.Count -eq 0 -and $warpSummary.allFreshEntriesRegistered) 'Server0.3.17 changed native evidence schema'
Assert (!$warpSummary.odysseyVerified -and !$warpSummary.battleVerified) 'stable-warp fixture fabricated live acceptance'
Write-Host 'Server0.3.17 stable-warp capture fixture passed; manual teleport/reentry acceptance remains pending.'

param(
 [Parameter(Mandatory=$true)][string]$Root,
 [ValidateSet('Check','Install','Rollback')][string]$Mode='Check'
)
$ErrorActionPreference='Stop'
Set-Location -LiteralPath $PSScriptRoot
function Hash([string]$Path) {
 if(Test-Path -LiteralPath $Path -PathType Leaf){return (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()}
 return $null
}
function SaveJson($Value,[string]$Path) {
 $temp=$Path+'.pending-'+[guid]::NewGuid().ToString('N')
 [IO.File]::WriteAllText($temp,($Value|ConvertTo-Json -Depth 30),[Text.UTF8Encoding]::new($false))
 if(Test-Path -LiteralPath $Path){[IO.File]::Replace($temp,$Path,[NullString]::Value)}else{[IO.File]::Move($temp,$Path)}
}
function CopyVerified([string]$Source,[string]$Destination,[string]$Expected) {
 [void][IO.Directory]::CreateDirectory((Split-Path -Parent $Destination))
 $temp=$Destination+'.update-'+[guid]::NewGuid().ToString('N')
 try {
  Copy-Item -LiteralPath $Source -Destination $temp
  if((Hash $temp) -ne $Expected){throw 'Temporary file hash mismatch.'}
  if(Test-Path -LiteralPath $Destination){[IO.File]::Replace($temp,$Destination,[NullString]::Value)}else{[IO.File]::Move($temp,$Destination)}
 }finally{if(Test-Path -LiteralPath $temp){Remove-Item -LiteralPath $temp}}
}
$locks=New-Object 'Collections.Generic.List[IO.FileStream]'
try {
 $m=Get-Content -LiteralPath (Join-Path $PSScriptRoot 'update-manifest.json') -Raw -Encoding UTF8|ConvertFrom-Json
 if($m.schema -ne 1 -or $m.release_id -notmatch '^[A-Za-z0-9][A-Za-z0-9._-]{0,95}$' -or $m.release_id.Contains('..')){throw 'Invalid update manifest.'}
 foreach($fingerprint in @($m.client_sha256,$m.mod_helper_sha256,$m.mod_policy_sha256,$m.publisher_sha256)){if($fingerprint -notmatch '^[0-9a-f]{64}$'){throw 'Missing or malformed package fingerprint.'}}
 $Root=(Resolve-Path -LiteralPath $Root.Trim().Trim('"')).Path.TrimEnd('\')
 $prefix=$Root+'\'
 function BoundedPath([string]$Path,[int]$Depth=0) {
  if($Depth -gt 24){throw 'Directory link cycle detected.'}
  $full=[IO.Path]::GetFullPath($Path)
  if($full -eq $Root){return $Root}
  if(!$full.StartsWith($prefix,[StringComparison]::OrdinalIgnoreCase)){throw 'Path escapes installation.'}
  $resolved=$Root
  foreach($segment in $full.Substring($prefix.Length).Split('\')){
   $resolved=Join-Path $resolved $segment
   if(Test-Path -LiteralPath $resolved){
    $item=Get-Item -LiteralPath $resolved -Force
    if($item.Attributes -band [IO.FileAttributes]::ReparsePoint){
     if(!$item.PSIsContainer){throw 'File reparse point is not supported by the update.'}
     $link=@($item.Target)[0]
     if(!$link){throw 'Unresolved directory link.'}
     $next=if([IO.Path]::IsPathRooted($link)){$link}else{Join-Path (Split-Path -Parent $resolved) $link}
     $resolved=BoundedPath $next ($Depth+1)
    }
   }
  }
  return $resolved
 }
 $client=$null
 $launcherConfig=Join-Path $Root 'server/launcher.local.json'
 if(Test-Path -LiteralPath $launcherConfig){
  [void](BoundedPath $launcherConfig)
  $launcherSettings=Get-Content -LiteralPath $launcherConfig -Raw -Encoding UTF8|ConvertFrom-Json
  if($launcherSettings.client_dir){
   $selected=[string]$launcherSettings.client_dir
   if(![IO.Path]::IsPathRooted($selected)){$selected=Join-Path (Join-Path $Root 'server') $selected}
   $selected=[IO.Path]::GetFullPath($selected)
   [void](BoundedPath $selected)
   if(!(Test-Path -LiteralPath (Join-Path $selected 'DFO.exe') -PathType Leaf)){throw 'Configured client has no DFO.exe.'}
   [void](BoundedPath (Join-Path $selected 'DFO.exe'));$client=$selected
  }
 }
 if(!$client){
  $clientIdentity=$null
  foreach($rel in @('client','DFO','01_客户端/DFO','01_客户端/client','01_客户端')){
   $candidate=Join-Path $Root $rel
   if(Test-Path -LiteralPath (Join-Path $candidate 'DFO.exe')){
    $identity=BoundedPath $candidate
    if($client -and $identity -ne $clientIdentity){throw 'Multiple client directories found; select one in launcher settings before updating.'}
    if(!$client){$client=$candidate;$clientIdentity=$identity}
   }
  }
 }
 if(!$client -and (Test-Path -LiteralPath (Join-Path $Root 'DFO.exe') -PathType Leaf)){$client=$Root}
 if($client){[void](BoundedPath (Join-Path $client 'DFO.exe'))}
 # The GUI may have a saved client selection not yet persisted by a game launch.
 $guiSettingsPath=Join-Path $Root 'local-launcher.settings.json'
 if(Test-Path -LiteralPath $guiSettingsPath -PathType Leaf){
  [void](BoundedPath $guiSettingsPath);$guiSettings=Get-Content -LiteralPath $guiSettingsPath -Raw -Encoding UTF8|ConvertFrom-Json
  $guiRoot=if($guiSettings.root){[string]$guiSettings.root}else{'.'}
  if(![IO.Path]::IsPathRooted($guiRoot)){$guiRoot=Join-Path $Root $guiRoot}
  if([IO.Path]::GetFullPath($guiRoot).TrimEnd('\') -eq $Root -and $guiSettings.client){
   $guiClient=[string]$guiSettings.client;if(![IO.Path]::IsPathRooted($guiClient)){$guiClient=Join-Path $Root $guiClient}
   if(!$client -or (BoundedPath $guiClient) -ne (BoundedPath $client)){throw 'Saved GUI client differs from the server launcher selection; apply the same client before updating.'}
  }
 }
 if(!$client -or (Hash (Join-Path $client 'DFO.exe')) -ne $m.client_sha256){throw 'Client build does not match this local release.'}
 function SafeTarget([string]$Rel) {
  $Rel=$Rel.Replace('\','/')
  if(!$Rel -or [IO.Path]::IsPathRooted($Rel) -or $Rel.Contains(':') -or $Rel -match '(^|/)\.\.?(/|$)' -or $Rel -match '(^|/)(runtime|pgdata)(/|$)' -or $Rel -match '\.(sqlite3?|db)(-|$)' -or $Rel -match '^client/') {throw 'Forbidden core update path.'}
  $dest=[IO.Path]::GetFullPath((Join-Path $Root $Rel))
  if(!$dest.StartsWith($prefix,[StringComparison]::OrdinalIgnoreCase)){throw 'Core target escapes installation.'}
  [void](BoundedPath $dest)
  return $dest
 }
 function AssertIdle {
  $names='^(DFO|dfolauncher|wireprobe.*|gm-local|gm-backend|DFO-115US本地版登录器|115US单机GM桌面版)\.exe$'
  foreach($p in @(Get-CimInstance Win32_Process|Where-Object {$_.Name -match $names})){
   if(!$p.ExecutablePath){
    $p=Get-CimInstance Win32_Process -Filter ('ProcessId='+$p.ProcessId)
    if(!$p -or $p.Name -notmatch $names){continue}
    if(!$p.ExecutablePath){throw 'A game or GM process cannot be identified; close it normally before updating.'}
   }
   $owned=$p.ExecutablePath.StartsWith($prefix,[StringComparison]::OrdinalIgnoreCase)
   $rootArgument='(?i)(?:^|\s)--?root(?:=|\s+)"?'+[regex]::Escape($Root)+'(?:"|\s|$)'
   if($owned -or ($p.CommandLine -and $p.CommandLine -match $rootArgument)){throw ('This installation is in use (PID '+$p.ProcessId+'). Exit normally before updating.')}
  }
 }
 $helper=Join-Path $PSScriptRoot 'ModUpgrade.ps1'
 $publisher=Join-Path $PSScriptRoot 'maintenance/Build-Server.ps1'
 if((Hash $helper) -ne $m.mod_helper_sha256){throw 'Mod upgrade helper hash mismatch.'}
 if((Hash (Join-Path $PSScriptRoot 'mod-upgrade/policy.json')) -ne $m.mod_policy_sha256){throw 'Mod upgrade policy hash mismatch.'}
 if((Hash $publisher) -ne $m.publisher_sha256){throw 'Gateway publisher hash mismatch.'}
 $backup=Join-Path $Root ('server/work/dfo-lan/runtime/update-backups/'+$m.release_id)
 [void](BoundedPath $backup)
 $receiptPath=Join-Path $backup 'receipt.json'
 function ReceiptModBackup($record) {
  if(!$record.mod_backup){throw 'Client rollback receipt path is missing.'}
  $path=[IO.Path]::GetFullPath([string]$record.mod_backup)
  $expected=[IO.Path]::GetFullPath((Join-Path $backup 'mods'))+'\'
  if(!$path.StartsWith($expected,[StringComparison]::OrdinalIgnoreCase)){throw 'Client receipt escapes update backup.'}
  [void](BoundedPath $path)
  return $path
 }
 function RestoreCore($row) {
  $dest=SafeTarget $row.path
  $actual=Hash $dest
  if($actual -eq $row.before){return}
  if($actual -ne $row.after){throw ('Unknown post-update edit; preserved: '+$row.path)}
  if($null -eq $row.before){Remove-Item -LiteralPath $dest}
  else {
   $source=Join-Path $backup ('files/'+$row.path)
   if((Hash $source) -ne $row.before){throw 'Original core backup hash mismatch.'}
   CopyVerified $source $dest $row.before
  }
 }
 function SafeClientTarget([string]$Rel) {
  if(!$Rel -or [IO.Path]::IsPathRooted($Rel) -or $Rel.Contains(':') -or $Rel -match '(^|[/\\])\.\.?([/\\]|$)'){throw 'Invalid client recovery baseline path.'}
  $path=[IO.Path]::GetFullPath((Join-Path $client $Rel))
  if(!$path.StartsWith($client.TrimEnd('\')+'\',[StringComparison]::OrdinalIgnoreCase)){throw 'Client recovery path escapes selected client.'}
  [void](BoundedPath $path);return $path
 }
 function ClientMutationPaths {
  $policy=Get-Content -LiteralPath (Join-Path $PSScriptRoot 'mod-upgrade/policy.json') -Raw -Encoding UTF8|ConvertFrom-Json
  $paths=@('.launcher-mods/modkit/registry.json','.launcher-mods/modkit/audit.jsonl')
  foreach($file in @($policy.files)){
   if(!$file){continue}
   if(!$file.target -or !$file.owner -or $file.owner -match '[/\\:]|^\.\.?$'){throw 'Invalid mod policy recovery target.'}
   $paths+=[string]$file.target
   $paths+=('.launcher-mods/modkit/backups/'+$file.owner+'/'+$file.target)
  }
  return @($paths | Sort-Object -Unique)
 }
 function AssertClientUnchanged($record) {
  if(!$record.client -or (BoundedPath ([string]$record.client)) -ne (BoundedPath $client) -or $record.client_policy_sha256 -ne $m.mod_policy_sha256){throw 'Client recovery identity or policy mismatch.'}
  $expected=@(ClientMutationPaths);$baseline=@{}
  foreach($entry in @($record.client_before)){
   if(!$entry.path -or $baseline.ContainsKey([string]$entry.path)){throw 'Incomplete or duplicate client recovery baseline.'}
   [void](SafeClientTarget ([string]$entry.path));$baseline[[string]$entry.path]=$entry.sha256
  }
  if($baseline.Count -ne $expected.Count){throw 'Incomplete client recovery baseline; manual review required.'}
  foreach($rel in $expected){if(!$baseline.ContainsKey($rel) -or (Hash (SafeClientTarget $rel)) -ne $baseline[$rel]){throw ('Client changed without a complete mod receipt; preserved: '+$rel)}}
 }
 function RestoreMods($record) {
  $modDir=ReceiptModBackup $record;$childPath=Join-Path $modDir 'receipt.json';$child=$null
  if(Test-Path -LiteralPath $childPath -PathType Leaf){[void](BoundedPath $childPath);$child=Get-Content -LiteralPath $childPath -Raw -Encoding UTF8|ConvertFrom-Json}
  if(!$child -or $child.status -eq 'preparing'){
   AssertClientUnchanged $record
   Write-Output 'No client mutation occurred before the mod snapshot; baseline hashes verified.'
   return
  }
  & $helper -Root $Root -PayloadRoot $PSScriptRoot -Mode Rollback -BackupRoot $modDir
  if($LASTEXITCODE -ne 0){throw 'Mod rollback needs recovery; core remains unchanged.'}
 }
 if($Mode -eq 'Rollback'){
  AssertIdle
  $receipt=Get-Content -LiteralPath $receiptPath -Raw -Encoding UTF8|ConvertFrom-Json
  if($receipt.root -ne $Root -or $receipt.release_id -ne $m.release_id){throw 'Receipt belongs to another installation.'}
  if($receipt.state -eq 'rolled-back'){Write-Output 'Already rolled back.';exit 0}
  if(@('preparing','installed','recovery-required') -notcontains $receipt.state){throw 'Unexpected receipt state.'}
  foreach($row in @($receipt.files)){
   $current=Hash (SafeTarget $row.path)
   if($current -ne $row.before -and $current -ne $row.after){throw ('Changed after installation: '+$row.path)}
  }
  RestoreMods $receipt
  $restoreRows=@($receipt.files);[array]::Reverse($restoreRows)
  foreach($row in $restoreRows){RestoreCore $row}
  $receipt.state='rolled-back';SaveJson $receipt $receiptPath
  Write-Output 'Programs and resources rolled back. Player data was not rolled back.';exit 0
 }
 # Launcher-generated manifests contain local paths and mtimes, so their whole-file
 # hash is not portable. Only this metadata file has a semantic fallback. Its four
 # fingerprints must match actual, independently allowlisted installation files.
 function TestGeneratedInnerManifest([string]$Path) {
  if(!(Test-Path -LiteralPath $Path -PathType Leaf) -or (Get-Item -LiteralPath $Path).Length -gt 65536){return $false}
  try{$doc=Get-Content -LiteralPath $Path -Raw -Encoding UTF8|ConvertFrom-Json}catch{return $false}
  if($doc -isnot [pscustomobject] -or $doc.format -cne 'dfo_20260901_inner' -or $doc.decoder -cne 'pvf_archive.wrapper_keys/aes'){return $false}
  foreach($key in $doc.PSObject.Properties.Name){if($key -cnotin @('format','decoder','client_exe','sk_dat','outer','inner','cache')){return $false}}
  $policy=Get-Content -LiteralPath (Join-Path $PSScriptRoot 'mod-upgrade/policy.json') -Raw -Encoding UTF8|ConvertFrom-Json
  $innerRows=@($m.files|Where-Object {$_.path -ceq 'server/work/client-build/Script.inner.pvf'})
  $skRows=@($policy.files|Where-Object {$_.target -ceq 'sk.dat'})
  $outerRows=@($policy.files|Where-Object {$_.target -ceq 'Script.pvf'})
  if($innerRows.Count -ne 1 -or $skRows.Count -ne 1 -or $outerRows.Count -ne 1){return $false}
  $inputs=@(
   @{key='client_exe';path=(Join-Path $client 'DFO.exe');allowed=@($m.client_sha256)},
   @{key='sk_dat';path=(Join-Path $client 'sk.dat');allowed=(@($skRows[0].sha256)+@($skRows[0].allowed_before))},
   @{key='outer';path=(Join-Path $client 'Script.pvf');allowed=(@($outerRows[0].sha256)+@($outerRows[0].allowed_before))},
   @{key='inner';path=(SafeTarget 'server/work/client-build/Script.inner.pvf');allowed=(@($innerRows[0].sha256)+@($innerRows[0].allowed_before))}
  )
  foreach($inputFile in $inputs){
   $record=$doc.($inputFile.key)
   if($record -isnot [pscustomobject]){return $false}
   if(@($record.PSObject.Properties).Count -ne 3){return $false}
   foreach($key in $record.PSObject.Properties.Name){if($key -cnotin @('path','size','sha256')){return $false}}
   if($record.path -isnot [string] -or [string]::IsNullOrWhiteSpace($record.path)){return $false}
   if(($record.size -isnot [int] -and $record.size -isnot [long]) -or $record.size -lt 0){return $false}
   if($record.sha256 -isnot [string] -or $record.sha256 -cnotmatch '^[0-9a-f]{64}$' -or $inputFile.allowed -cnotcontains $record.sha256){return $false}
   # Never resolve or read paths supplied by the JSON; use selected installation paths.
   [void](BoundedPath $inputFile.path)
   if(!(Test-Path -LiteralPath $inputFile.path -PathType Leaf)){return $false}
   if((Get-Item -LiteralPath $inputFile.path).Length -ne $record.size -or (Hash $inputFile.path) -cne $record.sha256){return $false}
  }
  if($doc.PSObject.Properties['cache'] -and $null -ne $doc.cache){
   if($doc.cache -isnot [array] -or $doc.cache.Count -ne 3){return $false}
   $cacheNames=@{}
   foreach($entry in $doc.cache){
    if($entry -isnot [pscustomobject] -or @($entry.PSObject.Properties).Count -ne 3){return $false}
    foreach($key in $entry.PSObject.Properties.Name){if($key -cnotin @('name','size','mtime_ns')){return $false}}
    if($entry.name -cnotin @('DFO.exe','sk.dat','Script.pvf') -or $cacheNames.ContainsKey($entry.name)){return $false}
    if(($entry.size -isnot [int] -and $entry.size -isnot [long]) -or $entry.size -lt 0){return $false}
    if($entry.mtime_ns -isnot [int] -and $entry.mtime_ns -isnot [long]){return $false}
    $cacheNames[$entry.name]=$true
   }
  }
  return $true
 }

 $rows=@();$seen=@{}
 foreach($f in @($m.files)){
  if($f.sha256 -notmatch '^[0-9a-f]{64}$' -or $f.bytes -lt 0){throw 'Invalid core payload fingerprint.'}
  $dest=SafeTarget $f.path
  if($seen.ContainsKey($dest.ToLowerInvariant())){throw 'Duplicate update target.'};$seen[$dest.ToLowerInvariant()]=$true
  $source=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot ('payload/'+$f.path)))
  $payloadPrefix=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot 'payload'))+'\'
  if(!$source.StartsWith($payloadPrefix,[StringComparison]::OrdinalIgnoreCase)){throw 'Payload path escapes package.'}
  if((Hash $source) -ne $f.sha256 -or (Get-Item -LiteralPath $source).Length -ne $f.bytes){throw ('Invalid payload: '+$f.path)}
  $before=Hash $dest
  if($before -ne $f.sha256 -and @($f.allowed_before) -notcontains $before){
   if($f.path -ceq 'server/work/client-build/Script.inner.manifest.json' -and (TestGeneratedInnerManifest $dest)){
    Write-Output 'Recognized launcher-generated PVF manifest; resource fingerprints verified. Original metadata will be backed up on install.'
   }else{throw ('Unrecognized installed file; preserved: '+$f.path)}
  }
  $rows+=@{path=[string]$f.path;before=$before;after=[string]$f.sha256;source=$source;target=$dest}
 }
 & $helper -Root $Root -PayloadRoot $PSScriptRoot -Mode Check -BackupRoot (Join-Path $backup 'mods')
 if($LASTEXITCODE -ne 0){throw 'Client update preflight failed.'}
 if($Mode -eq 'Check'){Write-Output ('Update preflight passed: '+$m.release_id);exit 0}
 AssertIdle
 if(Test-Path -LiteralPath $receiptPath){
  $prior=Get-Content -LiteralPath $receiptPath -Raw -Encoding UTF8|ConvertFrom-Json
  if($prior.state -eq 'installed' -and !@($rows|Where-Object {$_.before -ne $_.after}).Count){Write-Output 'This release is already installed.';exit 0}
  if($prior.state -ne 'rolled-back'){throw 'Previous update requires rollback before retry.'}
 }
 [void][IO.Directory]::CreateDirectory($backup)
 $module=Join-Path $Root 'server/work/dfo-lan'
 $storageConfig=Join-Path $module 'runtime/storage/local.json'
 $db=$null;$storageBefore=@{}
 if(Test-Path -LiteralPath $storageConfig){
  $cfg=Get-Content -LiteralPath $storageConfig -Raw -Encoding UTF8|ConvertFrom-Json
  if($cfg.driver -and $cfg.driver -ne 'sqlite'){throw 'Only the existing SQLite route is supported.'}
  if($cfg.postgres_dsn -and !$cfg.driver){throw 'Legacy database route requires separate review.'}
  $db=if($cfg.sqlite_path){[string]$cfg.sqlite_path}else{'runtime/storage/dfolan.sqlite3'}
  if(![IO.Path]::IsPathRooted($db)){$db=Join-Path $module $db};$db=[IO.Path]::GetFullPath($db)
  if(Test-Path -LiteralPath $db){
   $backupTool=@($rows|Where-Object {$_.path -eq 'server/work/dfo-lan/bin/dfolauncher.exe'})
   if($backupTool.Count -ne 1 -or (Hash $backupTool[0].source) -ne $backupTool[0].after){throw 'Verified backup tool is missing from this update.'}
   $previousPreference=$ErrorActionPreference
   $previousConsoleEncoding=[Console]::OutputEncoding
   try {
    $ErrorActionPreference='Continue'
    [Console]::OutputEncoding=[Text.UTF8Encoding]::new($false,$true)
    $backupOutput=& $backupTool[0].source storage-sync --root $Root --source storage --expected-database $db --backup
    $backupExit=$LASTEXITCODE
   }finally{
    try{[Console]::OutputEncoding=$previousConsoleEncoding}finally{$ErrorActionPreference=$previousPreference}
   }
   if($backupExit -ne 0){throw 'Reliable save backup failed; nothing installed.'}
   $backupResult=($backupOutput -join "`n")|ConvertFrom-Json
   if(!$backupResult.ok -or !$backupResult.backup.directory -or !@($backupResult.backup.files).Count){throw 'Save backup receipt is incomplete; nothing installed.'}
   $saveBackupDirectory=[IO.Path]::GetFullPath([string]$backupResult.backup.directory)
   if(!(Test-Path -LiteralPath (Join-Path $saveBackupDirectory 'manifest.json') -PathType Leaf)){throw 'Save backup manifest is missing; nothing installed.'}
   foreach($part in @($backupResult.backup.files)){
    if(!$part.name -or [IO.Path]::IsPathRooted($part.name) -or $part.name -match '(^|[/\\])\.\.([/\\]|$)' -or $part.name.Contains(':')){throw 'Invalid save backup entry.'}
    $partPath=[IO.Path]::GetFullPath((Join-Path $saveBackupDirectory $part.name))
    if(!$partPath.StartsWith($saveBackupDirectory+'\',[StringComparison]::OrdinalIgnoreCase) -or (Hash $partPath) -ne $part.sha256 -or (Get-Item -LiteralPath $partPath).Length -ne $part.size){throw 'Save backup data verification failed; nothing installed.'}
   }
   SaveJson $backupResult (Join-Path $backup 'storage-backup-receipt.json')
  }
  AssertIdle
  foreach($p in @($storageConfig,$db,($db+'-wal'),($db+'-shm'))){
   if(Test-Path -LiteralPath $p -PathType Leaf){
    $lock=[IO.File]::Open($p,[IO.FileMode]::Open,[IO.FileAccess]::Read,[IO.FileShare]::Read);$locks.Add($lock)
    $storageBefore[$p]=Hash $p
   }
  }
  Copy-Item -LiteralPath $storageConfig -Destination (Join-Path $backup 'preserved-local.json') -Force
 }
 $changes=@($rows|Where-Object {$_.before -ne $_.after})
 foreach($row in $changes){
  if($null -ne $row.before){CopyVerified $row.target (Join-Path $backup ('files/'+$row.path)) $row.before}
 }
 $modBackup=Join-Path $backup ('mods/'+[guid]::NewGuid().ToString('N'))
 $clientBefore=@(foreach($rel in @(ClientMutationPaths)){@{path=$rel;sha256=(Hash (SafeClientTarget $rel))}})
 $receipt=@{root=$Root;release_id=$m.release_id;state='preparing';mod_backup=$modBackup;client=$client;client_policy_sha256=$m.mod_policy_sha256;client_before=$clientBefore;files=@($changes|ForEach-Object {@{path=$_.path;before=$_.before;after=$_.after}});storage_before=$storageBefore;player_data_restored=$false}
 SaveJson $receipt $receiptPath
 $done=@();$modsStarted=$false
 try {
  AssertIdle
  $modsStarted=$true
  & $helper -Root $Root -PayloadRoot $PSScriptRoot -Mode Install -BackupRoot $modBackup
  if($LASTEXITCODE -ne 0){throw 'Client update failed.'}
  foreach($row in $changes){
   if((Hash $row.target) -ne $row.before){throw 'File changed during update.'}
   $done+=$row
   if($row.path -eq 'server/work/dfo-lan/bin/wireprobe-pvf.exe'){
    & $publisher -ReleaseRoot $Root -UpdatePVFDefault -AcceptedCandidate $row.source -AcceptedSHA256 $row.after -ExpectedDefaultSHA256 $row.before -DefaultBackup (Join-Path $backup ('gateway-'+[guid]::NewGuid().ToString('N')+'.exe')) -SkipEnvEnsure
    if($LASTEXITCODE -ne 0){throw 'Guarded gateway publication failed.'}
   }else{CopyVerified $row.source $row.target $row.after}
   if((Hash $row.target) -ne $row.after){throw 'Installed file hash mismatch.'}
  }
  foreach($p in @($storageBefore.Keys)){if((Hash $p) -ne $storageBefore[$p]){throw 'Protected storage file changed; no data rollback will be attempted.'}}
  $receipt.state='installed';$receipt.storage_preserved=$true;SaveJson $receipt $receiptPath
 }catch{
  $failure=$_.Exception.Message;$problems=@()
  [array]::Reverse($done)
  foreach($row in $done){try{RestoreCore $row}catch{$problems+=($_.Exception.Message)}}
  if($modsStarted){try{RestoreMods $receipt}catch{$problems+=($_.Exception.Message)}}
  $receipt.state=if($problems.Count){'recovery-required'}else{'rolled-back'}
  $receipt.error=$failure;$receipt.recovery_errors=$problems;SaveJson $receipt $receiptPath
  throw ($failure+' Backup: '+$backup+' '+($problems -join '; '))
 }
 Write-Output ('Installed '+$m.release_id+'. Save and configuration preserved. Receipt: '+$receiptPath)
 exit 0
}catch{Write-Error $_.Exception.Message;exit 1}
finally{foreach($lock in $locks){$lock.Dispose()}}

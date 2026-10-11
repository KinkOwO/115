param(
 [Parameter(Mandatory=$true)][string]$Root,
 [Parameter(Mandatory=$true)][string]$PayloadRoot,
 [Parameter(Mandatory=$true)][ValidateSet('Check','Install','Rollback')][string]$Mode,
 [Parameter(Mandatory=$true)][string]$BackupRoot
)
$ErrorActionPreference='Stop'
Set-Location -LiteralPath (Split-Path -Parent (Split-Path -Parent $PSScriptRoot))
Add-Type -AssemblyName System.IO.Compression
Add-Type -AssemblyName System.IO.Compression.FileSystem
$Utf8=New-Object Text.UTF8Encoding($false)
function Full([string]$p) { [IO.Path]::GetFullPath($p).TrimEnd('\') }
function RealDir([string]$p) { $i=Get-Item -LiteralPath $p -Force; if($i.LinkType -in @('Junction','SymbolicLink')) { $t=@($i.Target)[0];if(-not[IO.Path]::IsPathRooted($t)){$t=Join-Path $i.Parent.FullName $t};return Full $t };return Full $p }
function Under([string]$base,[string]$rel) {
 if ([string]::IsNullOrWhiteSpace($rel) -or [IO.Path]::IsPathRooted($rel) -or $rel.Contains(':') -or ($rel -split '[/\\]') -contains '..') { throw "Unsafe relative path: $rel" }
 $p=Full (Join-Path $base $rel)
 if (-not $p.StartsWith((Full $base)+'\',[StringComparison]::OrdinalIgnoreCase)) { throw "Path escapes root: $rel" }
 # Existing directory links must stay within the selected root. File symbolic links are refused.
 $realBase=if(Test-Path -LiteralPath $base){RealDir $base}else{Full $base}
 $walk=$p
 while($walk -and $walk-ne(Full $base)){
  if(Test-Path -LiteralPath $walk){$i=Get-Item -LiteralPath $walk -Force;if($i.Attributes-band[IO.FileAttributes]::ReparsePoint){
   if(-not$i.PSIsContainer){throw "File reparse point refused: $walk"};$resolved=RealDir $walk
   if($resolved-ne$realBase-and-not$resolved.StartsWith($realBase+'\',[StringComparison]::OrdinalIgnoreCase)){throw "Directory link escapes selected root: $walk"}
  }}
  $walk=[IO.Path]::GetDirectoryName($walk)
 }
 $p
}
function Hash([string]$p) { if (Test-Path -LiteralPath $p -PathType Leaf) { $h=[Security.Cryptography.SHA256]::Create();$f=[IO.File]::OpenRead($p);try{([BitConverter]::ToString($h.ComputeHash($f))).Replace('-','').ToLowerInvariant()}finally{$f.Dispose();$h.Dispose()} } else { '' } }
function EnsureDir([string]$p) { [IO.Directory]::CreateDirectory($p) | Out-Null }
function AtomicCopy([string]$src,[string]$dst) {
 EnsureDir ([IO.Path]::GetDirectoryName($dst));$tmp=$dst+'.integration-'+[Guid]::NewGuid().ToString('N')
 try{[IO.File]::Copy($src,$tmp,$false)
 if ([IO.File]::Exists($dst)) { [IO.File]::Replace($tmp,$dst,[NullString]::Value) } else { [IO.File]::Move($tmp,$dst) }
 }finally{if([IO.File]::Exists($tmp)){[IO.File]::Delete($tmp)}}
}
function WriteJson($value,[string]$path) {
 EnsureDir ([IO.Path]::GetDirectoryName($path));$tmp=$path+'.integration-'+[Guid]::NewGuid().ToString('N')
 try{[IO.File]::WriteAllText($tmp,($value | ConvertTo-Json -Depth 100)+"`n",$Utf8)
 if ([IO.File]::Exists($path)) { [IO.File]::Replace($tmp,$path,[NullString]::Value) } else { [IO.File]::Move($tmp,$path) }
 }finally{if([IO.File]::Exists($tmp)){[IO.File]::Delete($tmp)}}
}
function ReadJson([string]$p) { Get-Content -LiteralPath $p -Raw -Encoding UTF8 | ConvertFrom-Json }
function Clone($v) { $v | ConvertTo-Json -Depth 100 -Compress | ConvertFrom-Json }
function Canon($v) { $v | ConvertTo-Json -Depth 100 -Compress }
function SetProp($o,[string]$k,$v) { $o | Add-Member -NotePropertyName $k -NotePropertyValue $v -Force }
function NativeSame($old,$new,[bool]$entry=$false) {
 # The native Go registry writer omits extension properties. During interrupted
 # recovery only that exact projection is allowed, never a changed known field.
 $fields=if($entry){@('kind','target','work','backup','originalSHA256','installedSHA256','note')}else{@('id','name','version','manifestSHA256','sourceRoot','installedAt','requiresSnapshot','schema','layers','packageSHA256')}
 foreach($key in $fields){if((Canon $old.$key)-ne(Canon $new.$key)){if([string]$old.$key-ne[string]$new.$key){return $false}}}
 foreach($prop in $new.PSObject.Properties){if($prop.Name-notin$fields-and$prop.Name-ne'entries'){if(-not$old.PSObject.Properties[$prop.Name]-or(Canon $old.($prop.Name))-ne(Canon $prop.Value)){return $false}}}
 if(-not$entry){if(@($old.entries).Count-ne@($new.entries).Count){return $false};for($i=0;$i-lt@($old.entries).Count;$i++){if(-not(NativeSame @($old.entries)[$i] @($new.entries)[$i] $true)){return $false}}}
 return $true
}
function Registry([string]$client) { $p=Under $client '.launcher-mods/modkit/registry.json';if(Test-Path -LiteralPath $p){ReadJson $p}else{[pscustomobject]@{schema=1;mods=@()}} }
function FindMod($reg,[string]$id) { @($reg.mods | Where-Object {$_.id -eq $id}) | Select-Object -First 1 }
function FindEntry($mod,[string]$target) { @($mod.entries | Where-Object {$_.target -ieq $target}) | Select-Object -First 1 }
$Root=Full $Root;$PayloadRoot=Full $PayloadRoot;$BackupRoot=Full $BackupRoot
$policyPath=Join-Path $PayloadRoot 'mod-upgrade/policy.json';$Policy=ReadJson $policyPath;$PolicyHash=Hash $policyPath
$Tool=Join-Path $PayloadRoot 'mod-upgrade/modkit.exe'
if ((Hash $Tool) -ne $Policy.modkit_sha256) { throw 'modkit fingerprint mismatch' }
$candidates=@('client','DFO','01_客户端/DFO','01_客户端/client','01_客户端')
$Client=$null
$localConfig=Under $Root 'server/launcher.local.json'
if(Test-Path -LiteralPath $localConfig -PathType Leaf){
 $cfg=ReadJson $localConfig;$configured=[string]$cfg.client_dir
 if(-not[string]::IsNullOrWhiteSpace($configured)){
  $p=if([IO.Path]::IsPathRooted($configured)){Full $configured}else{Full (Join-Path (Join-Path $Root 'server') $configured)}
  if($p-ne$Root-and-not$p.StartsWith($Root+'\',[StringComparison]::OrdinalIgnoreCase)){throw 'External configured client is unsupported by this update'}
  if($p-ne$Root){$p=Under $Root $p.Substring($Root.Length+1)}
  if(-not(Test-Path -LiteralPath (Join-Path $p 'DFO.exe'))){throw 'Configured client has no DFO.exe'}
  $resolved=RealDir $p;$rootResolved=RealDir $Root
  if($resolved-ne$rootResolved-and-not$resolved.StartsWith($rootResolved+'\',[StringComparison]::OrdinalIgnoreCase)){throw 'External configured client is unsupported by this update'}
  $Client=$p
 }
}
if(-not$Client){foreach($r in $candidates){$p=Under $Root $r;if(Test-Path -LiteralPath (Join-Path $p 'DFO.exe')){if($Client -and (RealDir $Client) -ne (RealDir $p)){throw 'Ambiguous client roots'};if(-not$Client){$Client=$p}}}}
if(-not $Client -and (Test-Path -LiteralPath (Join-Path $Root 'DFO.exe'))){$Client=$Root}
if(-not $Client){throw 'Cannot locate client/DFO.exe'}
if((Hash (Join-Path $Client 'DFO.exe')) -ne $Policy.dfo_sha256){throw 'Unknown DFO.exe; no executable modifications are supported'}
$Journal=Join-Path $BackupRoot 'receipt.json'
function Native([string]$verb,[string]$package,[string]$tag) {
 $nativeArgs=@($verb,'--mod',$package)
 if($verb -ne 'verify'){$nativeArgs+=@('--client',$Client,'--root',$Root)}
 $log=Join-Path $BackupRoot ($tag+'-'+$verb+'.log')
 & $Tool @nativeArgs *> $log
 if($LASTEXITCODE -ne 0){throw "modkit $verb failed: $log"}
}
function Snapshot([string[]]$paths) {
 $list=@();foreach($rel in ($paths | Sort-Object -Unique)){
  $src=Under $Client $rel;$h=Hash $src;$save=Under (Join-Path $BackupRoot 'before') $rel
  if($h){AtomicCopy $src $save;if((Hash $save)-ne$h){throw "Backup verification failed: $rel"}}
  $list+= [pscustomobject]@{path=$rel;before=$h;after=$null}
 };return $list
}
function Restore($state,[bool]$strict) {
 if($state.client -ne $Client -or $state.policy_sha256 -ne $PolicyHash){throw 'Receipt target/policy mismatch'}
 # Verify every backup and every externally visible target before any restore.
 foreach($f in $state.files){
  if($f.before -and (Hash (Under (Join-Path $BackupRoot 'before') $f.path)) -ne $f.before){throw "Backup damaged: $($f.path)"}
  if($strict -and (Hash (Under $Client $f.path)) -ne $f.after){throw "Post-install change detected: $($f.path)"}
 }
 foreach($f in $state.files){
  $dst=Under $Client $f.path
  if($f.before){AtomicCopy (Under (Join-Path $BackupRoot 'before') $f.path) $dst}
  elseif(Test-Path -LiteralPath $dst -PathType Leaf){Remove-Item -LiteralPath $dst -Force}
 }
 foreach($f in $state.files){if((Hash (Under $Client $f.path))-ne$f.before){throw "Rollback verification failed: $($f.path)"}}
 $state.status='rolled_back';WriteJson $state $Journal
}
function CheckRecovery($state) {
 if($state.client-ne$Client -or $state.policy_sha256-ne$PolicyHash){throw 'Interrupted receipt target/policy mismatch'}
 $current=Registry $Client;$original=$state.original_registry
 $ids=@($Policy.owners.id)
 foreach($m in $current.mods){
  $old=FindMod $original $m.id
  if($ids-notcontains$m.id){if(-not$old-or-not(NativeSame $old $m)){throw "Unrelated owner changed during interruption: $($m.id)"};continue}
  $packages=@($state.packages|Where-Object{$_.owner-eq$m.id})
  $allowed=@($old.manifestSHA256)+@($packages.manifest_sha256);if(-not$old-and$packages.Count){$allowed+=@('')}
  if($allowed-notcontains$m.manifestSHA256){throw "Unknown interrupted manifest: $($m.id)"}
  $changed=@($Policy.files|Where-Object{$_.owner-eq$m.id})
  foreach($e in $m.entries){
   $oldEntry=FindEntry $old $e.target
   if($changed.target-notcontains$e.target){if(-not$oldEntry-or-not(NativeSame $oldEntry $e $true)){throw "Unchanged entry changed during interruption: $($e.target)"}}
   else{
    $f=@($changed|Where-Object{$_.target-eq$e.target})[0];$snap=@($state.files|Where-Object{$_.path-eq$e.target})[0]
    if(@($snap.before,$f.sha256,$oldEntry.installedSHA256,'')-notcontains[string]$e.installedSHA256){throw "Unknown interrupted entry fingerprint: $($e.target)"}
    if($e.backup-and$e.backup-ne('backups/'+$m.id+'/'+$e.target)-and$e.backup-ne$oldEntry.backup){throw "Unknown interrupted backup reference: $($e.target)"}
   }
  }
 }
 foreach($m in $original.mods){if(-not(FindMod $current $m.id)){throw "Owner removed during interruption: $($m.id)"}}
 foreach($prop in $current.PSObject.Properties){if($prop.Name-ne'mods'){if(-not$original.PSObject.Properties[$prop.Name]-or(Canon $original.($prop.Name))-ne(Canon $prop.Value)){throw "Registry root metadata changed: $($prop.Name)"}}}
 foreach($f in $state.files){
  if($f.path-in@('.launcher-mods/modkit/registry.json','.launcher-mods/modkit/audit.jsonl')){continue}
  $allowed=@($f.before);$payload=@($Policy.files|Where-Object{$_.target-eq$f.path})|Select-Object -First 1
  if($payload){$allowed+=@($payload.sha256)}
  foreach($p in $Policy.files){if($f.path-eq('.launcher-mods/modkit/backups/'+$p.owner+'/'+$p.target)){$s=@($state.files|Where-Object{$_.path-eq$p.target})[0];$allowed+=@($s.before)}}
  if($allowed-notcontains(Hash (Under $Client $f.path))){throw "Unknown interrupted file content: $($f.path)"}
 }
}
if($Mode -eq 'Rollback'){
 if(-not(Test-Path -LiteralPath $Journal)){throw 'Missing receipt'}
 $s=ReadJson $Journal
 if($s.status -eq 'rolled_back'){Write-Output 'Already rolled back';exit 0}
 if($s.status -eq 'installed'){Restore $s $true}
 elseif($s.status-in@('prepared','installing')){CheckRecovery $s;Restore $s $false}
 else{throw 'No completed pre-install snapshot; no rollback is needed before the prepared stage'}
 Write-Output 'Client files, ownership and prior backups restored byte for byte';exit 0
}
$Reg=Registry $Client
if($Reg.schema -ne 1){throw 'Unsupported mod registry schema'}
$ownerByTarget=@{};$recordByID=@{}
foreach($m in $Reg.mods){
 if($recordByID.ContainsKey($m.id)){throw "Duplicate owner: $($m.id)"};$recordByID[$m.id]=$m
 foreach($e in $m.entries){$null=Under $Client $e.target;if($ownerByTarget.ContainsKey($e.target)){throw "Duplicate target owner: $($e.target)"};$ownerByTarget[$e.target]=$m.id}
}
$definitions=@{};foreach($o in $Policy.owners){$definitions[$o.id]=$o}
$targets=@{};$locks=@();$actions=@();$affected=@{}
foreach($f in $Policy.files){
 if($targets.ContainsKey($f.target)){throw "Duplicate policy target: $($f.target)"};$targets[$f.target]=$f
 $src=Under $PayloadRoot $f.payload;$dst=Under $Client $f.target
 if((Hash $src)-ne$f.sha256 -or (Get-Item -LiteralPath $src).Length-ne$f.bytes){throw "Payload corrupt: $($f.target)"}
 $h=Hash $dst;if(@($f.allowed_before)-notcontains$h -and $h-ne$f.sha256){throw "Unknown current file: $($f.target) [$h]"}
 if($ownerByTarget.ContainsKey($f.target) -and $ownerByTarget[$f.target]-ne$f.owner){throw "Unknown owner on $($f.target): $($ownerByTarget[$f.target])"}
 $locks+= [pscustomobject]@{path=$f.target;sha256=$h};$affected[$f.owner]=$true
}
foreach($id in $affected.Keys){
 $def=$definitions[$id];if(-not$def){throw "Missing owner policy: $id"}
 $m=$recordByID[$id]
 if($m){
  $known=@($def.allowed_manifest_sha256)-contains$m.manifestSHA256
  if(-not$known -and $m.version-ne$Policy.version){throw "Unknown owner version/manifest: $id"}
  if((Canon @($m.requiresSnapshot | Sort-Object)) -ne (Canon @($def.requires | Sort-Object))){throw "Unknown dependency change: $id"}
  foreach($e in $m.entries){
   $knownEntry=@($def.known_entries | Where-Object {$_.target-ieq$e.target -and (NativeSame $_ $e $true)}) | Select-Object -First 1
   if(-not$knownEntry -and -not($targets.ContainsKey($e.target)-and$targets[$e.target].owner-eq$id)){throw "Unknown extra entry under $id : $($e.target)"}
   if($known-and$knownEntry-and-not(NativeSame $knownEntry $e $true)){throw "Historical owner entry metadata changed: $($e.target)"}
   if($e.kind-notin@('file.add','file.replace','exe.patch')){throw "Unsupported entry kind: $($e.kind)"}
   $actual=Hash (Under $Client $e.target)
   if(-not$targets.ContainsKey($e.target)){
    if($actual-ne$e.installedSHA256 -or $actual-ne$knownEntry.installedSHA256){throw "Unchanged owner entry drift: $($e.target)"}
    $locks+= [pscustomobject]@{path=$e.target;sha256=$actual}
   }elseif($e.installedSHA256-ne$targets[$e.target].sha256 -and @($knownEntry.installedSHA256)-notcontains$e.installedSHA256){throw "Unknown registered installed hash: $($e.target)"}
   if($e.backup){
    $rel='.launcher-mods/modkit/'+$e.backup;$bh=Hash (Under $Client $rel)
    if($bh-ne$e.originalSHA256){throw "Owner backup missing or corrupt: $rel"}
    $locks+=[pscustomobject]@{path=$rel;sha256=$bh}
   }
  }
 }
 foreach($dep in $def.requires){if(-not$recordByID.ContainsKey($dep)-and-not$affected.ContainsKey($dep)){throw "Missing dependency: $dep"}}
}
$ordered=@();$pending=@($Policy.owners | Where-Object {$affected.ContainsKey($_.id)})
while($pending.Count){$ready=@($pending|Where-Object{$d=$_;@($d.requires|Where-Object{$affected.ContainsKey($_)-and$ordered.id-notcontains$_}).Count-eq0});if(-not$ready.Count){throw 'Dependency cycle'};foreach($o in $ready){$ordered+=$o};$pending=@($pending|Where-Object{$ordered.id-notcontains$_.id})}
$locks+=[pscustomobject]@{path='.launcher-mods/modkit/registry.json';sha256=(Hash (Under $Client '.launcher-mods/modkit/registry.json'))}
$plan=[pscustomobject]@{version=$Policy.version;client=$Client;policy_sha256=$PolicyHash;owners=@($ordered.id);targets=@($Policy.files.target);locks=$locks;dfo_never_written=$true}
if($Mode-eq'Check'){$plan|ConvertTo-Json -Depth 12;exit 0}
EnsureDir $BackupRoot
WriteJson $plan (Join-Path $BackupRoot 'check-plan.json')
if(Test-Path -LiteralPath $Journal){
 $prior=ReadJson $Journal
 if($prior.status-eq'installed'){throw 'Already installed with this receipt; use Rollback or a new receipt directory'}
 if($prior.status-ne'rolled_back'){throw 'Interrupted receipt exists; preserve it and recover using the transaction evidence'}
 throw 'Use a fresh backup directory after rollback'
}
# Snapshot all only-client targets and exactly the registry/audit/backup paths modkit can touch.
$paths=@($Policy.files.target)+@('.launcher-mods/modkit/registry.json','.launcher-mods/modkit/audit.jsonl')
foreach($f in $Policy.files){$paths+=('.launcher-mods/modkit/backups/'+$f.owner+'/'+$f.target)}
$state=[pscustomobject]@{schema=1;status='preparing';client=$Client;root=$Root;policy_sha256=$PolicyHash;original_registry=$Reg;files=@();packages=@();completed_owners=@()}
WriteJson $state $Journal
$state.files=@(Snapshot $paths);$state.status='prepared';WriteJson $state $Journal
foreach($l in $locks){if((Hash (Under $Client $l.path))-ne$l.sha256){throw "Concurrent change before install: $($l.path)"}}
$state.status='installing';WriteJson $state $Journal
try {
 foreach($o in $ordered){
  $id=$o.id;$old=FindMod $Reg $id;$oldEntries=@($old.entries);$owned=@($Policy.files|Where-Object{$_.owner-eq$id})
  $ops=@();foreach($f in $owned){$kind='file.add';if(Hash (Under $Client $f.target)){$kind='file.replace'};$ops+=[ordered]@{kind=$kind;target=$f.target;source=('client/'+$f.target);size=$f.bytes;sha256=$f.sha256}}
  $manifest=[ordered]@{schema=2;id=$id;version=$Policy.version;name=$id;description='Verified 20261010 integration delta; omitted historical entries and original backups remain owned and unchanged.';permissions=@('client.file.write');requires=@($o.requires);layers=@{client=@{ops=$ops}}}
  $pkg=Join-Path $BackupRoot ('packages/'+$id+'.zip');EnsureDir ([IO.Path]::GetDirectoryName($pkg))
  $z=[IO.Compression.ZipFile]::Open($pkg,[IO.Compression.ZipArchiveMode]::Create)
  try{
   $entry=$z.CreateEntry('mod.json',[IO.Compression.CompressionLevel]::NoCompression);$entry.LastWriteTime=[DateTimeOffset]'2026-10-10T00:00:00Z';$stream=$entry.Open();$b=$Utf8.GetBytes(($manifest|ConvertTo-Json -Depth 30)+"`n");$stream.Write($b,0,$b.Length);$stream.Dispose()
   foreach($f in $owned){$entry=$z.CreateEntry(('client/'+$f.target),[IO.Compression.CompressionLevel]::NoCompression);$entry.LastWriteTime=[DateTimeOffset]'2026-10-10T00:00:00Z';$memberInput=[IO.File]::OpenRead((Under $PayloadRoot $f.payload));$memberOutput=$entry.Open();try{$memberInput.CopyTo($memberOutput)}finally{$memberInput.Dispose();$memberOutput.Dispose()}}
  }finally{$z.Dispose()}
  $mh=[Security.Cryptography.SHA256]::Create();try{$manifestHash=([BitConverter]::ToString($mh.ComputeHash($b))).Replace('-','').ToLowerInvariant()}finally{$mh.Dispose()}
  $state.packages+= [pscustomobject]@{owner=$id;path=$pkg;sha256=(Hash $pkg);manifest_sha256=$manifestHash;manifest=$manifest};WriteJson $state $Journal
  Native 'verify' $pkg $id;Native 'plan' $pkg $id;Native 'install' $pkg $id
  $now=Registry $Client;$rec=FindMod $now $id;if(-not$rec){throw "Native record missing: $id"}
  foreach($prop in $old.PSObject.Properties){if($prop.Name-notin@('id','name','version','manifestSHA256','sourceRoot','installedAt','requiresSnapshot','entries','schema','layers','packageSHA256')){SetProp $rec $prop.Name $prop.Value}}
  $merged=@($oldEntries|Where-Object{$_})
  foreach($f in $owned){
   if((Hash (Under $Client $f.target))-ne$f.sha256){throw "Installed file mismatch: $($f.target)"}
   $e=FindEntry $old $f.target
   if($e){
    $e=Clone $e;SetProp $e 'installedSHA256' $f.sha256
    # Native replacement overwrites the owner backup; retain the prior owner's undo chain.
    $bk='.launcher-mods/modkit/backups/'+$id+'/'+$f.target
    $snap=@($state.files|Where-Object{$_.path-eq$bk})[0]
    if($snap.before){AtomicCopy (Under (Join-Path $BackupRoot 'before') $bk) (Under $Client $bk)}
    elseif(Test-Path -LiteralPath (Under $Client $bk)){Remove-Item -LiteralPath (Under $Client $bk) -Force}
   }else{
    $s=@($state.files|Where-Object{$_.path-eq$f.target})[0]
    $e=[pscustomobject]@{kind='file.add';target=$f.target;installedSHA256=$f.sha256;note='Integration ownership established from verified schema2 payload and observed pre-install state'}
    if($s.before){
     $bk='backups/'+$id+'/'+$f.target;AtomicCopy (Under (Join-Path $BackupRoot 'before') $f.target) (Under $Client ('.launcher-mods/modkit/'+$bk))
     SetProp $e 'kind' 'file.replace';SetProp $e 'backup' $bk;SetProp $e 'originalSHA256' $s.before
    }
   }
   $merged=@($merged|Where-Object{$_.target-ine$f.target})+@($e)
  }
  SetProp $rec 'entries' $merged
  # Keep the exact unrelated owner objects, including fields unknown to the native tool.
  $out=Clone $Reg;$out.mods=@($Reg.mods|ForEach-Object{if($_.id-eq$id){$rec}else{$_}})
  if(-not$old){$out.mods+=@($rec)}
  WriteJson $out (Under $Client '.launcher-mods/modkit/registry.json');$Reg=$out
  $state.completed_owners+= $id;WriteJson $state $Journal
 }
 foreach($f in $state.files){$f.after=Hash (Under $Client $f.path)}
 foreach($l in $locks){if(-not$targets.ContainsKey($l.path)-and$l.path-ne'.launcher-mods/modkit/registry.json'-and(Hash (Under $Client $l.path))-ne$l.sha256){throw "Unchanged file/backup altered: $($l.path)"}}
 foreach($f in $Policy.files){$r=FindMod $Reg $f.owner;$e=FindEntry $r $f.target;if(-not$e-or$e.installedSHA256-ne$f.sha256){throw "Final owner fingerprint mismatch: $($f.target)"}}
 $state.status='installed';WriteJson $state $Journal;Write-Output 'Client integration installed; registry owners and prior backups preserved'
}catch{
 $cause=$_;try{CheckRecovery $state;Restore $state $false}catch{Write-Warning "Automatic rollback refused or failed; unknown changes preserved: $_"};throw $cause
}

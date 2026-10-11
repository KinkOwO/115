param([Parameter(Mandatory=$true)][string]$Root,[ValidateSet('Check','Install','Rollback')][string]$Mode='Check')
$ErrorActionPreference='Stop'
Set-Location -LiteralPath (Split-Path -Parent (Split-Path -Parent $PSScriptRoot))
Set-Location -LiteralPath $PSScriptRoot
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

function Hash([string]$p){if(Test-Path -LiteralPath $p -PathType Leaf){(Get-FileHash -LiteralPath $p -Algorithm SHA256).Hash.ToLowerInvariant()}else{''}}
function ReadJson([string]$p){Get-Content -LiteralPath $p -Raw -Encoding UTF8|ConvertFrom-Json}
$lease=$null
try {
 $Root=Full (Resolve-Path -LiteralPath $Root).Path
 $helper=Join-Path $PSScriptRoot 'ModUpgrade.ps1'
 if((Hash $helper)-ne'4e0cd9d9c76d3fee247a8be640ed164a42c7646903cc2ba40933c514728bab88'){throw '可选安装器组件校验失败。'}
 if((Hash (Join-Path $PSScriptRoot 'mod-upgrade/policy.json'))-ne'c5ec3de71ec48f484926c9c51179e590b16665276c24ed5e900f2e42b189999d'){throw '可选安装清单校验失败。'}
 # Let the same verified helper resolve the client/configuration and enforce
 # ownership, dependency, symlink and known-byte checks before any write.
 $temporaryBackup=Join-Path $Root '.optional-input-check-unused'
 $raw=& powershell.exe -NoProfile -ExecutionPolicy Bypass -File $helper -Root $Root -PayloadRoot $PSScriptRoot -Mode Check -BackupRoot $temporaryBackup
 if($LASTEXITCODE-ne0){throw '可选输入组件兼容检查未通过。'}
 $plan=($raw -join "`n")|ConvertFrom-Json
 $client=[string]$plan.client
 $target=Under $client '.115us-mods/ChineseInputProbe.dll'
 if($Mode-ne'Rollback'){
  if((Hash (Join-Path $client 'dinput8.dll'))-ne'5b4a90994d2ba59441f7f96d3dc6bc37a40c3f8e9267db38e43b41fbc64b1147'-or(Hash (Join-Path $client 'ChineseLocalization.dll'))-ne'90ee6bbc01b0c8e506e2b9a6ced7bb79234610fa1da4b0ca00f99b56a498832b'){throw '请先安装本次累计更新；当前宿主或加载器不匹配。'}
 }
 if($Mode-eq'Check'){Write-Output '可选中文输入兼容检查通过；不会替换宿主、加载器或其他插件。';exit 0}
 $prefix=$client.TrimEnd('')+''
 if(@(Get-CimInstance Win32_Process|Where-Object{$_.Name-eq'DFO.exe'-and$_.ExecutablePath-and$_.ExecutablePath.StartsWith($prefix,[StringComparison]::OrdinalIgnoreCase)}).Count){throw '请先正常退出游戏。'}
 $history=Under $client '.launcher-mods/optional-input-cumulative-r3'
 [void][IO.Directory]::CreateDirectory($history)
 $lease=[IO.File]::Open((Join-Path $history 'operation.lock'),[IO.FileMode]::OpenOrCreate,[IO.FileAccess]::ReadWrite,[IO.FileShare]::None)
 $pointer=Join-Path $history 'active.json';$prior=$null;$backup=$null
 if(Test-Path -LiteralPath $pointer){$a=ReadJson $pointer;if([string]$a.id-notmatch'^[a-f0-9]{32}$'){throw '可选修复记录异常。'};$backup=Under $history ([string]$a.id);$prior=ReadJson (Join-Path $backup 'receipt.json')}
 if($Mode-eq'Rollback'){
  if(-not$prior){throw '没有本次可选附件的安装记录。'}
  & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $helper -Root $Root -PayloadRoot $PSScriptRoot -Mode Rollback -BackupRoot $backup
  if($LASTEXITCODE-ne0){throw '回退被拒绝，请保留记录并查看上方原因。'}
  Write-Output '已还原安装前的输入插件及登记记录。';exit 0
 }
 if($prior-and$prior.status-eq'installed'){
  if((Hash $target)-ne'6eca78d7222608ce9627fc5530bb8727db1d9f1b4e1dd4577c9320f7d44cfd1c'){throw '已安装的输入插件发生变化，未覆盖。'}
  Write-Output '已安装，无需重复安装。';exit 0
 }
 if($prior-and$prior.status-ne'rolled_back'){throw '上次安装尚未完成，请先回退后重试。'}
 $id=[guid]::NewGuid().ToString('N');$backup=Under $history $id
 # Persist recovery location before the helper performs its first mutation.
 $pending=$pointer+'.pending';[IO.File]::WriteAllText($pending,(@{id=$id}|ConvertTo-Json),[Text.UTF8Encoding]::new($false))
 if(Test-Path -LiteralPath $pointer){[IO.File]::Replace($pending,$pointer,[NullString]::Value)}else{[IO.File]::Move($pending,$pointer)}
 & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $helper -Root $Root -PayloadRoot $PSScriptRoot -Mode Install -BackupRoot $backup
 if($LASTEXITCODE-ne0){throw '可选安装未完成，请保留记录并查看上方原因。'}
 Write-Output '可选中文输入插件已安装；实际输入法表现需要在游戏内确认。'
}catch{Write-Error $_.Exception.Message;exit 1}finally{if($lease){$lease.Dispose()}}

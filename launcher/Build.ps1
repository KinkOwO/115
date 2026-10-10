param(

 [Parameter(Mandatory=$true)][string]$Backend,

 [Parameter(Mandatory=$true)][string]$Server,

 [string]$ReleaseId='2026.10.11-cumulative-r5',
 [string]$OutDir=''

)

$ErrorActionPreference='Stop'

if(-not $OutDir){$OutDir=Join-Path $PSScriptRoot 'dist'}

New-Item -ItemType Directory -Force -Path $OutDir | Out-Null

$taskFramework=Join-Path $env:SystemRoot 'Microsoft.NET\Framework64\v4.0.30319'

$taskCompiler=Join-Path $taskFramework 'csc.exe'

if($ReleaseId -notmatch '^[A-Za-z0-9._-]+$'){throw 'Invalid release ID'}

$taskServerHash=(Get-FileHash -LiteralPath $Server -Algorithm SHA256).Hash.ToLowerInvariant()

$taskGMHash=(Get-FileHash -LiteralPath $Backend -Algorithm SHA256).Hash.ToLowerInvariant()

$taskGenerated=Join-Path $OutDir 'GeneratedRelease.cs'

$taskCode=@"

using System.Reflection;

[assembly: AssemblyTitle("115US Local + GM Launcher")]

[assembly: AssemblyVersion("2.0.5.0")]

[assembly: AssemblyFileVersion("2.0.5.0")]

[assembly: AssemblyInformationalVersion("$ReleaseId")]

namespace Local115US { public static class ReleaseBuild {

 public const string ID="$ReleaseId";

 public const string ServerHash="$taskServerHash";

 public const string GMHash="$taskGMHash";

} }

"@

[IO.File]::WriteAllText($taskGenerated,$taskCode,[Text.UTF8Encoding]::new($false))

$taskLauncher=Join-Path $PSScriptRoot 'src\launcher'

$taskOutput=Join-Path $OutDir 'DFO-115US本地版登录器.exe'

$taskArgs=@('/nologo','/target:winexe','/main:Local115US.Program','/platform:x64','/optimize+','/utf8output',('/out:'+$taskOutput),('/win32icon:'+(Join-Path $taskLauncher 'DFO.ico')),('/win32manifest:'+(Join-Path $taskLauncher 'app.manifest')),('/resource:'+(Join-Path $taskLauncher 'Shell.xaml')+',Shell.xaml'),('/resource:'+(Join-Path $taskLauncher 'hero.jpg')+',hero.jpg'),('/resource:'+(Join-Path $taskLauncher 'DFO.ico')+',app.ico'),('/resource:'+$Backend+',backend.exe'))

foreach($taskRef in @('System.dll','System.Core.dll','System.Drawing.dll','System.Security.dll','System.Web.Extensions.dll','System.Windows.Forms.dll','System.Xaml.dll','WPF\WindowsBase.dll','WPF\PresentationCore.dll','WPF\PresentationFramework.dll','WPF\WindowsFormsIntegration.dll')){$taskArgs+='/reference:'+(Join-Path $taskFramework $taskRef)}

$taskArgs+=$taskGenerated

Get-ChildItem -LiteralPath $taskLauncher,(Join-Path $PSScriptRoot 'src\gm') -Filter '*.cs' -File | ForEach-Object {$taskArgs+=$_.FullName}

& $taskCompiler @taskArgs

if($LASTEXITCODE -ne 0){throw 'Unified launcher compilation failed'}

$taskRedirect=Join-Path $OutDir '115US单机GM桌面版.exe'

& $taskCompiler /nologo /target:winexe /platform:x64 /optimize+ /utf8output ('/out:'+$taskRedirect) ('/reference:'+(Join-Path $taskFramework 'System.dll')) ('/reference:'+(Join-Path $taskFramework 'System.Windows.Forms.dll')) (Join-Path $PSScriptRoot 'tools\LegacyGMRedirect.cs')

if($LASTEXITCODE -ne 0){throw 'Legacy GM redirect compilation failed'}

$taskResult=[ordered]@{release_id=$ReleaseId;version='2.0.5.0';server_sha256=$taskServerHash;embedded_gm_sha256=$taskGMHash;launcher=$taskOutput;launcher_sha256=(Get-FileHash -LiteralPath $taskOutput).Hash.ToLowerInvariant();legacy_gm_redirect=$taskRedirect;legacy_gm_redirect_sha256=(Get-FileHash -LiteralPath $taskRedirect).Hash.ToLowerInvariant();built_utc=[DateTime]::UtcNow.ToString('o')}

$taskResult | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $OutDir 'build-result.json') -Encoding utf8

$taskResult|ConvertTo-Json

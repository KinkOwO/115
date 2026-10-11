$ErrorActionPreference='Stop'
$taskRoot=Split-Path $PSScriptRoot -Parent
$taskDist=Join-Path $taskRoot 'dist'
$taskCompiler=Join-Path $env:SystemRoot 'Microsoft.NET\Framework64\v4.0.30319\csc.exe'
& $taskCompiler /nologo /target:exe /platform:x64 ('/out:'+(Join-Path $taskDist 'LifecycleWorker.exe')) (Join-Path $PSScriptRoot 'LifecycleWorker.cs')
if($LASTEXITCODE -ne 0){throw 'worker compile failed'}
& $taskCompiler /nologo /target:exe /platform:x64 ('/out:'+(Join-Path $taskDist 'LifecycleCheck.exe')) ('/reference:'+(Join-Path $taskDist 'DFO-115US本地版登录器.exe')) /reference:System.Windows.Forms.dll /reference:System.Drawing.dll (Join-Path $PSScriptRoot 'LifecycleCheck.cs')
if($LASTEXITCODE -ne 0){throw 'lifecycle test compile failed'}
$taskEvidence=Join-Path $taskRoot 'evidence'
$taskProcess=Start-Process -FilePath (Join-Path $taskDist 'LifecycleCheck.exe') -ArgumentList ('"'+$taskEvidence+'"') -WindowStyle Hidden -Wait -PassThru -RedirectStandardOutput (Join-Path $taskEvidence 'lifecycle.stdout.log') -RedirectStandardError (Join-Path $taskEvidence 'lifecycle.stderr.log')
Get-Content -LiteralPath (Join-Path $taskEvidence 'lifecycle.stdout.log'),(Join-Path $taskEvidence 'lifecycle.stderr.log')
if($taskProcess.ExitCode -ne 0){throw 'lifecycle test failed'}

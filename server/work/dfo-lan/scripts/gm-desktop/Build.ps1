param([Parameter(Mandatory=$true)][string]$Backend,[Parameter(Mandatory=$true)][string]$Output)
Import-Module (Join-Path $PSHOME 'Modules/Microsoft.PowerShell.Utility/Microsoft.PowerShell.Utility.psd1')
$ErrorActionPreference='Stop'
$module=Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
Set-Location -LiteralPath $module
$framework=Join-Path $env:SystemRoot 'Microsoft.NET\Framework64\v4.0.30319'
$compiler=Join-Path $framework 'csc.exe'
New-Item -ItemType Directory -Path $Output -Force | Out-Null
# Generate the tool's own icon from vector geometry; keep build assets in Output.
Add-Type -AssemblyName System.Drawing
$iconFile=Join-Path $Output 'gm-mark.ico'
$bitmap=New-Object System.Drawing.Bitmap 48,48
$drawing=[System.Drawing.Graphics]::FromImage($bitmap)
$drawing.SmoothingMode=[System.Drawing.Drawing2D.SmoothingMode]::AntiAlias
$drawing.Clear([System.Drawing.Color]::FromArgb(23,28,35))
$ink=[System.Drawing.Color]::FromArgb(232,190,111)
$pen=New-Object System.Drawing.Pen $ink,2
[System.Drawing.Point[]]$points=@([System.Drawing.Point]::new(24,3),[System.Drawing.Point]::new(43,13),[System.Drawing.Point]::new(43,35),[System.Drawing.Point]::new(24,45),[System.Drawing.Point]::new(5,35),[System.Drawing.Point]::new(5,13))
$drawing.DrawPolygon($pen,$points)
$font=New-Object System.Drawing.Font 'Segoe UI',12,([System.Drawing.FontStyle]::Bold)
$brush=New-Object System.Drawing.SolidBrush $ink
$format=New-Object System.Drawing.StringFormat
$format.Alignment=[System.Drawing.StringAlignment]::Center
$format.LineAlignment=[System.Drawing.StringAlignment]::Center
$drawing.DrawString('GM',$font,$brush,[System.Drawing.RectangleF]::new(0,0,48,48),$format)
$pngStream=New-Object System.IO.MemoryStream
$bitmap.Save($pngStream,[System.Drawing.Imaging.ImageFormat]::Png)
$pngBytes=$pngStream.ToArray()
$iconStream=[System.IO.File]::Create($iconFile)
$writer=New-Object System.IO.BinaryWriter $iconStream
try{
 $writer.Write([UInt16]0);$writer.Write([UInt16]1);$writer.Write([UInt16]1)
 $writer.Write([byte]48);$writer.Write([byte]48);$writer.Write([byte]0);$writer.Write([byte]0)
 $writer.Write([UInt16]1);$writer.Write([UInt16]32);$writer.Write([UInt32]$pngBytes.Length);$writer.Write([UInt32]22)
 $writer.Write($pngBytes)
}finally{$writer.Dispose();$pngStream.Dispose();$drawing.Dispose();$bitmap.Dispose();$font.Dispose();$brush.Dispose();$pen.Dispose();$format.Dispose()}
$refs=@('System.dll','System.Core.dll','System.Drawing.dll','System.Windows.Forms.dll','System.Security.dll','System.Web.Extensions.dll')
foreach($edition in @('local')){
 $name=if($edition -eq 'local'){'115US单机GM桌面版.exe'}else{'115US联网GM桌面版.exe'}
 $argsList=@('/nologo','/target:winexe','/platform:x64','/optimize+','/utf8output',('/out:'+(Join-Path $Output $name)))
 foreach($ref in $refs){$argsList+='/reference:'+(Join-Path $framework $ref)}
 $argsList+=('/resource:'+$Backend+',backend.exe')
 $argsList+=('/win32icon:'+$iconFile)
 $argsList+=('/resource:'+$iconFile+',app.ico')
 if($edition -eq 'cloud'){$argsList+='/define:CLOUD'}
 $argsList+=(Join-Path $PSScriptRoot 'Desktop.cs')
 $argsList+=(Join-Path $PSScriptRoot 'Advanced.cs')
 $argsList+=(Join-Path $PSScriptRoot 'Theme.cs')
 $argsList+=(Join-Path $PSScriptRoot 'Design.cs')
 $argsList+=(Join-Path $PSScriptRoot 'OwnerProfile.cs')
 $argsList+=(Join-Path $PSScriptRoot 'Players.cs')
 $argsList+=(Join-Path $PSScriptRoot 'LocalMaintenance.cs')
 & $compiler @argsList
 if($LASTEXITCODE -ne 0){throw ('Desktop build failed: '+$edition)}
}
Write-Output 'Native GM desktop programs built successfully.'

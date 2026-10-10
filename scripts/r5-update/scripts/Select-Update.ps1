param([ValidateSet('Check','Install','Rollback')][string]$Mode='Check',[string]$Root)
$ErrorActionPreference='Stop'
if(!$Root){
 Add-Type -AssemblyName System.Windows.Forms
 $dialog=New-Object Windows.Forms.FolderBrowserDialog
 $dialog.Description='选择现有115US安装根目录（内含server与DFO或client）。'
 $dialog.ShowNewFolderButton=$false
 if(Test-Path -LiteralPath 'D:\115us'){$dialog.SelectedPath='D:\115us'}
 try{if($dialog.ShowDialog() -ne [Windows.Forms.DialogResult]::OK){Write-Output '已取消，未修改游戏。';exit 0};$Root=$dialog.SelectedPath}finally{$dialog.Dispose()}
}
& (Join-Path (Split-Path -Parent $PSScriptRoot) 'Update.ps1') -Root $Root -Mode $Mode
exit $LASTEXITCODE

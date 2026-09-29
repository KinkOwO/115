$base = 'http://127.0.0.1:28081'
foreach ($slot in 10..14) {
  $body = @{account=1; character=1; reason="清理测试装备行"; edits=@(@{section="equipment"; slot=$slot; durability=0})} | ConvertTo-Json -Depth 6
  try {
    $r = Invoke-WebRequest -Uri ($base + '/api/state/bag?token=x') -Method Post -Body $body -ContentType 'application/json' -UseBasicParsing -TimeoutSec 10
    $j = $r.Content | ConvertFrom-Json
    Write-Output ("slot {0} -> applied={1} msg={2}" -f $slot, $j.applied, $j.message)
  } catch {
    if ($_.Exception.Response) {
      $code = [int]$_.Exception.Response.StatusCode
      $sr = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
      Write-Output ("slot {0} -> REJECTED {1}: {2}" -f $slot, $code, $sr.ReadToEnd())
    } else { Write-Output ("slot {0} -> ERR: {1}" -f $slot, $_.Exception.Message) }
  }
  Start-Sleep -Milliseconds 300
}
Write-Output "--- 清理后背包 ---"
$r2 = Invoke-WebRequest -Uri ($base + '/api/character?id=1&token=x') -UseBasicParsing -TimeoutSec 10
($r2.Content | ConvertFrom-Json).bag | ForEach-Object { Write-Output ("sec=$($_.section) slot=$($_.slot) id=$($_.template) dur=$($_.durability)") }

$ErrorActionPreference = 'Continue'
$base = 'http://127.0.0.1:28081'
function Post-Json($path, $obj) {
  try {
    $body = $obj | ConvertTo-Json -Depth 8
    $sep = if ($path.Contains('?')) { '&' } else { '?' }
    $r = Invoke-WebRequest -Uri ($base + $path + $sep + 'token=x') -Method Post -Body $body -ContentType 'application/json' -UseBasicParsing -TimeoutSec 15
    return "HTTP " + $r.StatusCode + " " + $r.Content
  } catch {
    if ($_.Exception.Response) {
      $code = [int]$_.Exception.Response.StatusCode
      $sr = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
      $txt = $sr.ReadToEnd()
      return "REJECTED HTTP " + $code + ": " + $txt
    }
    return "ERR: " + $_.Exception.Message
  }
}
function Get-Json($path) {
  $sep = if ($path.Contains('?')) { '&' } else { '?' }
  $r = Invoke-WebRequest -Uri ($base + $path + $sep + 'token=x') -UseBasicParsing -TimeoutSec 10
  return ($r.Content | ConvertFrom-Json)
}

Write-Output "=== 1. 新增 items 行 (slot 0, 108000001 x5) ==="
Write-Output (Post-Json '/api/state/bag' @{account=1; character=1; reason="堆叠测试-items"; edits=@(@{section="items"; slot=0; template=108000001; amount=5})})
Start-Sleep -Milliseconds 800
$j = Get-Json '/api/character?id=1'
Write-Output "--- 背包现状 ---"
$j.bag | ForEach-Object { Write-Output ("sec=$($_.section) slot=$($_.slot) id=$($_.template) amt=$($_.amount) dur=$($_.durability)") }

Write-Output "=== 2. 同格 +3 (slot 0, amount 5->8) ==="
Write-Output (Post-Json '/api/state/bag' @{account=1; character=1; reason="堆叠测试-merge"; edits=@(@{section="items"; slot=0; amount=8})})
Start-Sleep -Milliseconds 800
$j2 = Get-Json '/api/character?id=1'
$row = $j2.bag | Where-Object { $_.section -eq 'items' }
Write-Output ("items row: slot=$($row.slot) id=$($row.template) amt=$($row.amount)")

Write-Output "=== 3. 减少 (slot 0, amount 8->3) ==="
Write-Output (Post-Json '/api/state/bag' @{account=1; character=1; reason="堆叠测试-sub"; edits=@(@{section="items"; slot=0; amount=3})})
Start-Sleep -Milliseconds 800
$j3 = Get-Json '/api/character?id=1'
$row3 = $j3.bag | Where-Object { $_.section -eq 'items' }
Write-Output ("items row: slot=$($row3.slot) id=$($row3.template) amt=$($row3.amount)")

Write-Output "=== 4. 清空 (slot 0, amount 0) ==="
Write-Output (Post-Json '/api/state/bag' @{account=1; character=1; reason="堆叠测试-del"; edits=@(@{section="items"; slot=0; amount=0})})
Start-Sleep -Milliseconds 800
$j4 = Get-Json '/api/character?id=1'
$items = @($j4.bag | Where-Object { $_.section -eq 'items' })
Write-Output ("items rows after del: " + $items.Count)

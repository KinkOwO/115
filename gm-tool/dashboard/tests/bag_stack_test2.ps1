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
function Show-Items {
  $j = Get-Json '/api/character?id=1'
  $rows = @($j.bag | Where-Object { $_.section -eq 'items' })
  if ($rows.Count) {
    $rows | ForEach-Object { Write-Output ("items slot=$($_.slot) id=$($_.template) amt=$($_.amount) name=$($_.name)") }
  } else {
    Write-Output "items 分区为空"
  }
}

Write-Output "=== 1. 发放材料 10300000 x2 ==="
Write-Output (Post-Json '/api/grant' @{account=1; character=1; gold=0; cera=0; reason="堆叠测试-A"; items=@(@{template=10300000; amount=2})})
Start-Sleep -Milliseconds 600
Show-Items

Write-Output "=== 2. 再发 10300000 x3 (期望合并为 5) ==="
Write-Output (Post-Json '/api/grant' @{account=1; character=1; gold=0; cera=0; reason="堆叠测试-B"; items=@(@{template=10300000; amount=3})})
Start-Sleep -Milliseconds 600
Show-Items

Write-Output "=== 3. 用 state/bag 把该行 amount 改为 4 (减少) ==="
$j3 = Get-Json '/api/character?id=1'
$row = @($j3.bag | Where-Object { $_.section -eq 'items' })[0]
Write-Output ("目标行 slot=$($row.slot) template=$($row.template)")
Write-Output (Post-Json '/api/state/bag' @{account=1; character=1; reason="堆叠测试-C"; edits=@(@{section="items"; slot=$row.slot; amount=4})})
Start-Sleep -Milliseconds 600
Show-Items

Write-Output "=== 4. 用 state/bag 删除该行 (amount 0) ==="
Write-Output (Post-Json '/api/state/bag' @{account=1; character=1; reason="堆叠测试-D"; edits=@(@{section="items"; slot=$row.slot; amount=0})})
Start-Sleep -Milliseconds 600
Show-Items

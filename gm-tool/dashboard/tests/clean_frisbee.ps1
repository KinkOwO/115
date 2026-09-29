$body = @{account=1; character=1; reason="清理 e2e 误发放"; edits=@(@{section="items"; slot=65; amount=0})} | ConvertTo-Json -Depth 6
$r = Invoke-WebRequest -Uri 'http://127.0.0.1:28081/api/state/bag?token=x' -Method Post -Body $body -ContentType 'application/json' -UseBasicParsing -TimeoutSec 10
$r.Content
""
$r2 = Invoke-WebRequest -Uri 'http://127.0.0.1:28081/api/character?id=1&token=x' -UseBasicParsing -TimeoutSec 10
"items rows: " + (($r2.Content | ConvertFrom-Json).bag | Where-Object { $_.section -eq 'items' }).Count
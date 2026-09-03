# Full workspace-webhook E2E against local contrib stack.
$ErrorActionPreference = 'Stop'
$base = 'http://127.0.0.1:8080'
$secret = 'whsec_contrib_e2e_secret_32ch'
$recv = 'http://127.0.0.1:18081/hooks/weknora'
$logFile = 'E:\Tencent\WeKnora-slj\tmp-webhook-e2e.jsonl'
$report = [ordered]@{}
$fail = 0

function Assert-True($cond, $name, $detail = '') {
  if ($cond) {
    Write-Host "PASS $name"
    $script:report[$name] = 'PASS'
  } else {
    Write-Host "FAIL $name $detail"
    $script:report[$name] = "FAIL $detail"
    $script:fail++
  }
}

function Api($method, $path, $token = $null, $body = $null, $headersExtra = $null) {
  $headers = @{ 'Content-Type' = 'application/json' }
  if ($token) { $headers.Authorization = "Bearer $token" }
  if ($headersExtra) { foreach ($k in $headersExtra.Keys) { $headers[$k] = $headersExtra[$k] } }
  $params = @{ Method = $method; Uri = "$base$path"; Headers = $headers }
  if ($null -ne $body) { $params.Body = ($body | ConvertTo-Json -Depth 8 -Compress) }
  return Invoke-RestMethod @params
}

function Wait-Deliveries($token, $tenantId, $hookId, $minCount, $timeoutSec = 45, $wantSuccessType = $null) {
  $deadline = (Get-Date).AddSeconds($timeoutSec)
  $data = @()
  do {
    $rows = Api GET "/api/v1/tenants/$tenantId/event/webhooks/$hookId/deliveries?limit=50" $token
    $data = @($rows.data)
    if ($wantSuccessType) {
      $hit = @($data | Where-Object { $_.event_type -eq $wantSuccessType -and $_.status -eq 'success' })
      if ($hit.Count -ge 1) { return $data }
    } elseif ($data.Count -ge $minCount) {
      return $data
    }
    Start-Sleep -Seconds 2
  } while ((Get-Date) -lt $deadline)
  return $data
}

function Wait-LogTypes([string[]]$want, $timeoutSec = 45) {
  $deadline = (Get-Date).AddSeconds($timeoutSec)
  do {
    $lines = @()
    if (Test-Path $logFile) {
      $lines = Get-Content $logFile | Where-Object { $_ } | ForEach-Object { $_ | ConvertFrom-Json }
    }
    $okTypes = @($lines | Where-Object { $_.ok } | ForEach-Object { if ($_.body_type) { $_.body_type } else { $_.event } })
    $missing = @($want | Where-Object { $_ -notin $okTypes })
    if ($missing.Count -eq 0) { return $lines }
    Start-Sleep -Seconds 2
  } while ((Get-Date) -lt $deadline)
  return $lines
}

# --- auth ---
$login = Api POST '/api/v1/auth/login' $null @{ email = 'owner@local.test'; password = 'Owner123456' }
$token = $login.token
$tenantId = $login.active_tenant.id
if (-not $tenantId) { $tenantId = $login.user.tenant_id }
Assert-True ($token -and $tenantId) 'login' "tenant=$tenantId"

# cleanup existing e2e hooks
$existing = Api GET "/api/v1/tenants/$tenantId/event/webhooks" $token
foreach ($ep in @($existing.data)) {
  if ($ep.name -like 'e2e-*') {
    Api DELETE "/api/v1/tenants/$tenantId/event/webhooks/$($ep.id)" $token | Out-Null
  }
}

# empty events rejected
try {
  Api POST "/api/v1/tenants/$tenantId/event/webhooks" $token @{
    name = 'e2e-empty'; url = $recv; secret = $secret; events = @()
  } | Out-Null
  Assert-True $false 'reject_empty_events'
} catch {
  Assert-True ($_.Exception.Response.StatusCode.value__ -ge 400) 'reject_empty_events' $_.Exception.Message
}

# non-loopback http rejected
try {
  Api POST "/api/v1/tenants/$tenantId/event/webhooks" $token @{
    name = 'e2e-http'; url = 'http://example.com/hooks'; secret = $secret
    events = @('webhook.test')
  } | Out-Null
  Assert-True $false 'reject_non_loopback_http'
} catch {
  Assert-True ($_.Exception.Response.StatusCode.value__ -ge 400) 'reject_non_loopback_http'
}

$allEvents = @(
  'knowledge.created','knowledge.parse_completed','knowledge.parse_failed',
  'knowledge.deleted','knowledge.batch_deleted','kb.created','kb.deleted',
  'rbac.member_added','rbac.member_removed'
)

$hook = Api POST "/api/v1/tenants/$tenantId/event/webhooks" $token @{
  name = 'e2e-main'
  url = $recv
  secret = $secret
  events = $allEvents
  enabled = $true
  description = 'full e2e'
}
$hookId = $hook.data.id
Assert-True ([bool]$hookId) 'create_endpoint' ($hook | ConvertTo-Json -Compress)

# webhook.test
Api POST "/api/v1/tenants/$tenantId/event/webhooks/$hookId/test" $token | Out-Null
$d1 = Wait-Deliveries $token $tenantId $hookId 1 45 'webhook.test'
$testOk = @($d1 | Where-Object { $_.event_type -eq 'webhook.test' -and $_.status -eq 'success' }).Count -ge 1
Assert-True $testOk 'deliver_webhook_test' (($d1 | Select-Object -First 3 | ConvertTo-Json -Compress))

# kb create/delete
$kb = Api POST '/api/v1/knowledge-bases' $token @{ name = "e2e-kb-$(Get-Date -Format 'HHmmss')"; type = 'document' }
$kbId = $kb.data.id
Assert-True ([bool]$kbId) 'kb_create' ($kb | ConvertTo-Json -Compress)
Api DELETE "/api/v1/knowledge-bases/$kbId" $token | Out-Null

# member add/remove
$memberEmail = "e2e-member-$(Get-Date -Format 'yyyyMMddHHmmss')@example.com"
try {
  Api POST '/api/v1/auth/register' $null @{
    username = ("m" + (Get-Random -Maximum 999999))
    email = $memberEmail
    password = 'Member123456'
  } | Out-Null
} catch {
  # already exists is fine
}
$added = Api POST "/api/v1/tenants/$tenantId/members" $token @{ email = $memberEmail; role = 'viewer' }
$memberUserId = $added.data.user_id
if (-not $memberUserId) { $memberUserId = $added.data.user.id }
Assert-True ([bool]$memberUserId) 'member_add' ($added | ConvertTo-Json -Compress)
Api DELETE "/api/v1/tenants/$tenantId/members/$memberUserId" $token | Out-Null

$want = @('webhook.test','kb.created','kb.deleted','rbac.member_added','rbac.member_removed')
$logRows = Wait-LogTypes $want 60
$okTypes = @($logRows | Where-Object { $_.ok } | ForEach-Object { if ($_.body_type) { $_.body_type } else { $_.event } } | Select-Object -Unique)
foreach ($t in $want) {
  Assert-True ($t -in $okTypes) "hmac_$t" ("got=" + ($okTypes -join ','))
}

$dAll = Wait-Deliveries $token $tenantId $hookId 5 60
$successTypes = @($dAll | Where-Object { $_.status -eq 'success' } | ForEach-Object { $_.event_type } | Select-Object -Unique)
foreach ($t in $want) {
  Assert-True ($t -in $successTypes) "delivery_$t" ("got=" + ($successTypes -join ','))
}

# ticket auth paths
$kid = '00000000-0000-0000-0000-000000000001'
try {
  Invoke-WebRequest -Uri "$base/api/v1/files/knowledge-download/$kid" -Method GET -TimeoutSec 5 | Out-Null
  Assert-True $false 'ticket_missing_401'
} catch {
  Assert-True ($_.Exception.Response.StatusCode.value__ -eq 401) 'ticket_missing_401' $_.Exception.Message
}
try {
  Invoke-WebRequest -Uri "$base/api/v1/files/knowledge-download/$kid" -Method GET -Headers @{ 'X-WeKnora-Download-Ticket' = 'wdt1.fake.sig' } -TimeoutSec 5 | Out-Null
  Assert-True $false 'ticket_fake_401'
} catch {
  Assert-True ($_.Exception.Response.StatusCode.value__ -eq 401) 'ticket_fake_401'
}
try {
  Invoke-WebRequest -Uri "$base/api/v1/files/knowledge-download/$kid/renew" -Method POST -TimeoutSec 5 | Out-Null
  Assert-True $false 'ticket_renew_missing_401'
} catch {
  Assert-True ($_.Exception.Response.StatusCode.value__ -eq 401) 'ticket_renew_missing_401'
}

# subscription gate: narrow existing endpoint to rbac.member_added only → kb.created must not write outbox
Api PATCH "/api/v1/tenants/$tenantId/event/webhooks/$hookId" $token @{
  events = @('rbac.member_added')
  enabled = $true
} | Out-Null
# warm subscription index briefly
Start-Sleep -Seconds 1
$before = (docker exec weknora-oss-postgres psql -U weknora -d weknora -tAc "SELECT COUNT(*) FROM tenant_webhook_outbox WHERE owner_tenant_id=$tenantId AND event_type='kb.created';").Trim()
$kb2 = Api POST '/api/v1/knowledge-bases' $token @{ name = "e2e-gate-$(Get-Date -Format 'HHmmss')"; type = 'document' }
Start-Sleep -Seconds 3
$after = (docker exec weknora-oss-postgres psql -U weknora -d weknora -tAc "SELECT COUNT(*) FROM tenant_webhook_outbox WHERE owner_tenant_id=$tenantId AND event_type='kb.created';").Trim()
$kb2Id = $kb2.data.id
if ($kb2Id) { Api DELETE "/api/v1/knowledge-bases/$kb2Id" $token | Out-Null }
Assert-True (([int]$after) -eq ([int]$before)) 'gate_no_outbox_unsubscribed_kb_created' "before=$before after=$after"

# cleanup hooks
Api DELETE "/api/v1/tenants/$tenantId/event/webhooks/$hookId" $token | Out-Null

Write-Host ''
Write-Host '==== E2E SUMMARY ===='
$report.GetEnumerator() | ForEach-Object { Write-Host ("{0}: {1}" -f $_.Key, $_.Value) }
Write-Host ("TOTAL_FAIL=$fail")
if ($fail -gt 0) { exit 1 } else { exit 0 }

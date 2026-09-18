param(
    [string]$Prometheus   = 'http://localhost:9090',
    [string]$Alertmanager = 'http://localhost:9093',
    [string]$Jaeger       = 'http://localhost:16686',
    [string]$Karma        = 'http://localhost:8081',
    [string]$Api          = 'http://localhost:8088',
    [string]$Grafana      = 'http://localhost:3000'
)

function Test-Step {
    param([string]$Name, [scriptblock]$Check)
    try {
        $result = & $Check
        if ($result) {
            Write-Host ("[ OK ] {0}: {1}" -f $Name, $result) -ForegroundColor Green
        } else {
            Write-Host ("[FAIL] {0}: пусто" -f $Name) -ForegroundColor Red
        }
    } catch {
        Write-Host ("[FAIL] {0}: {1}" -f $Name, $_.Exception.Message) -ForegroundColor Red
    }
}

function Invoke-Promql {
    param([string]$Query)
    (Invoke-RestMethod -Uri "$Prometheus/api/v1/query" -Body @{ query = $Query } -TimeoutSec 15).data.result
}

Write-Host '--- сервис ---' -ForegroundColor Cyan
Test-Step 'api /health' { (Invoke-WebRequest "$Api/health" -UseBasicParsing -TimeoutSec 10).Content }
Test-Step 'api отдаёт метрики' {
    $m = (Invoke-WebRequest "$Api/actuator/prometheus" -UseBasicParsing -TimeoutSec 10).Content
    "$(($m -split "`n" | Select-String '^http_server_requests_seconds_bucket').Count) бакетов гистограммы"
}

Write-Host '--- метрики ---' -ForegroundColor Cyan
Test-Step 'скрейп api живой (up)' { (Invoke-Promql 'sum(up{job="api"})').value[1] + ' реплик' }
Test-Step 'RPS' { '{0:N2} req/s' -f [double](Invoke-Promql 'sum(rate(http_server_requests_seconds_count{job="api"}[5m]))').value[1] }
Test-Step 'доля 5xx' {
    $q = 'sum(rate(http_server_requests_seconds_count{job="api",outcome="SERVER_ERROR"}[5m])) / sum(rate(http_server_requests_seconds_count{job="api"}[5m]))'
    '{0:P2}' -f [double](Invoke-Promql $q).value[1]
}
Test-Step 'p95' {
    $v = (Invoke-Promql 'histogram_quantile(0.95, sum by (le) (rate(http_server_requests_seconds_bucket{job="api",uri!~"/actuator.*"}[5m])))').value[1]
    '{0:N3} s' -f [double]$v
}
Test-Step 'правила загружены' {
    $rules = (Invoke-RestMethod "$Prometheus/api/v1/rules" -TimeoutSec 15).data.groups | Where-Object { $_.name -eq 'api.slo' }
    ($rules.rules | ForEach-Object { "$($_.name)=$($_.state)" }) -join ', '
}

Write-Host '--- логи ---' -ForegroundColor Cyan
Test-Step 'Grafana видит datasource Loki' {
    $h = @{ Authorization = 'Basic ' + [Convert]::ToBase64String([Text.Encoding]::ASCII.GetBytes('admin:admin')) }
    $ds = Invoke-RestMethod "$Grafana/api/datasources" -Headers $h -TimeoutSec 15
    ($ds | ForEach-Object { $_.uid }) -join ', '
}
Test-Step 'в Loki есть логи api с trace_id' {
    $h = @{ Authorization = 'Basic ' + [Convert]::ToBase64String([Text.Encoding]::ASCII.GetBytes('admin:admin')) }
    $q = [uri]::EscapeDataString('{namespace="obs", app="api"} |= "trace_id"')
    $r = Invoke-RestMethod "$Grafana/api/datasources/proxy/uid/loki/loki/api/v1/query_range?query=$q&limit=5" -Headers $h -TimeoutSec 20
    "$($r.data.result.Count) стримов"
}

Write-Host '--- трейсы ---' -ForegroundColor Cyan
Test-Step 'Jaeger знает сервис api' {
    ((Invoke-RestMethod "$Jaeger/api/services" -TimeoutSec 15).data) -join ', '
}
Test-Step 'есть трейс со вложенным span slow-op' {
    $r = Invoke-RestMethod "$Jaeger/api/traces?service=api&operation=slow-op&limit=20" -TimeoutSec 20
    $count = @($r.data).Count
    if ($count -eq 0) { return $null }
    "$count трейсов со slow-op"
}
Test-Step 'есть трейс с ошибкой' {
    $r = Invoke-RestMethod "$Jaeger/api/traces?service=api&tags=%7B%22error%22%3A%22true%22%7D&limit=20" -TimeoutSec 20
    $count = @($r.data).Count
    if ($count -eq 0) { return $null }
    "$count трейсов с error=true"
}
Test-Step 'trace_id из лога открывается в Jaeger' {
    $h = @{ Authorization = 'Basic ' + [Convert]::ToBase64String([Text.Encoding]::ASCII.GetBytes('admin:admin')) }
    $q = [uri]::EscapeDataString('{namespace="obs", app="api"} |= "trace_id"')
    $logs = Invoke-RestMethod "$Grafana/api/datasources/proxy/uid/loki/loki/api/v1/query_range?query=$q&limit=30" -Headers $h -TimeoutSec 20
    $ids = foreach ($stream in $logs.data.result) {
        foreach ($v in $stream.values) { ($v[1] | ConvertFrom-Json).trace_id }
    }
    foreach ($id in ($ids | Where-Object { $_ } | Select-Object -Unique)) {
        try {
            $trace = Invoke-RestMethod "$Jaeger/api/traces/$id" -TimeoutSec 15
            if ($trace.data) { return "$id -> $(@($trace.data[0].spans).Count) спанов" }
        } catch { }
    }
    return $null
}

Write-Host '--- алерты ---' -ForegroundColor Cyan
Test-Step 'Alertmanager принимает алерты' {
    $a = Invoke-RestMethod "$Alertmanager/api/v2/alerts" -TimeoutSec 15
    ($a | ForEach-Object { "$($_.labels.alertname)/$($_.status.state)" }) -join ', '
}
Test-Step 'Karma отвечает' { (Invoke-WebRequest "$Karma/health" -UseBasicParsing -TimeoutSec 10).StatusCode }

$ns = 'obs'

$forwards = @(
    @{ name = 'grafana';      target = 'svc/grafana';          ports = '3000:80' },
    @{ name = 'prometheus';   target = 'svc/kps-prometheus';   ports = '9090:9090' },
    @{ name = 'alertmanager'; target = 'svc/kps-alertmanager'; ports = '9093:9093' },
    @{ name = 'jaeger';       target = 'svc/jaeger-query';     ports = '16686:16686' },
    @{ name = 'karma';        target = 'svc/karma';            ports = '8081:8080' },
    @{ name = 'api';          target = 'svc/api';              ports = '8088:8080' }
)

Get-Job -Name 'pf-*' -ErrorAction SilentlyContinue | Stop-Job -PassThru | Remove-Job

foreach ($f in $forwards) {
    Start-Job -Name "pf-$($f.name)" -ScriptBlock {
        param($ns, $target, $ports)
        while ($true) {
            $started = Get-Date
            kubectl -n $ns port-forward $target $ports 2>&1 | ForEach-Object { "$_" }
            "[{0}] kubectl port-forward {1} завершился с кодом {2} через {3:N0} с, перезапуск" -f `
                (Get-Date -Format 'HH:mm:ss'), $target, $LASTEXITCODE, ((Get-Date) - $started).TotalSeconds
            Start-Sleep -Seconds 3
        }
    } -ArgumentList $ns, $f.target, $f.ports | Out-Null
    Write-Host ("{0,-13} http://localhost:{1}" -f $f.name, ($f.ports -split ':')[0])
}

Write-Host ''
Write-Host 'Grafana: admin / admin' -ForegroundColor Yellow
Write-Host 'Перезапуски пробросов: Receive-Job -Name pf-* -Keep | Select-String "завершился"' -ForegroundColor DarkGray

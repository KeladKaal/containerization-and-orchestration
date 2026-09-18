$ErrorActionPreference = 'Stop'

function Invoke-Native {
    param(
        [Parameter(Mandatory)][string]$Name,
        [Parameter(Mandatory)][scriptblock]$Command
    )
    $previous = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        & $Command
        if ($LASTEXITCODE -ne 0) {
            throw "$Name — код выхода $LASTEXITCODE"
        }
    } finally {
        $ErrorActionPreference = $previous
    }
}

$root = Split-Path -Parent $PSScriptRoot
$deploy = Join-Path $root 'deploy'
$cluster = 'obs'
$ns = 'obs'

Write-Host '==> kind cluster' -ForegroundColor Cyan
$previous = $ErrorActionPreference
$ErrorActionPreference = 'Continue'
$existing = kind get clusters 2>$null
$ErrorActionPreference = $previous
if ($existing -notcontains $cluster) {
    Invoke-Native 'kind create cluster' { kind create cluster --config (Join-Path $deploy 'kind-cluster.yaml') }
} else {
    Write-Host "кластер $cluster уже есть"
}
Invoke-Native 'kubectl config' { kubectl config use-context "kind-$cluster" }

Write-Host '==> сборка образа api' -ForegroundColor Cyan
Invoke-Native 'docker build' { docker build -t lab2/api:0.1.0 (Join-Path $root 'api') }
Invoke-Native 'kind load' { kind load docker-image lab2/api:0.1.0 --name $cluster }

Write-Host '==> helm repos' -ForegroundColor Cyan
Invoke-Native 'helm repo add prometheus-community' { helm repo add prometheus-community https://prometheus-community.github.io/helm-charts }
Invoke-Native 'helm repo add grafana' { helm repo add grafana https://grafana.github.io/helm-charts }
Invoke-Native 'helm repo update' { helm repo update }

Write-Host '==> kube-prometheus-stack (Prometheus + Alertmanager + Grafana)' -ForegroundColor Cyan
Invoke-Native 'helm kps' {
    helm upgrade --install kps prometheus-community/kube-prometheus-stack `
        --version 90.1.1 `
        --namespace $ns --create-namespace `
        -f (Join-Path $deploy 'values\kube-prometheus-stack.yaml') `
        --wait --timeout 15m
}

Write-Host '==> Loki' -ForegroundColor Cyan
Invoke-Native 'helm loki' {
    helm upgrade --install loki grafana/loki `
        --version 7.3.0 `
        --namespace $ns `
        -f (Join-Path $deploy 'values\loki.yaml') `
        --wait --timeout 15m
}

Write-Host '==> Alloy (сбор логов, DaemonSet)' -ForegroundColor Cyan
Invoke-Native 'helm alloy' {
    helm upgrade --install alloy grafana/alloy `
        --version 1.12.1 `
        --namespace $ns `
        -f (Join-Path $deploy 'values\alloy.yaml') `
        --wait --timeout 10m
}

Write-Host '==> Jaeger' -ForegroundColor Cyan
Invoke-Native 'helm jaeger' {
    helm upgrade --install jaeger (Join-Path $deploy 'charts\jaeger') --namespace $ns --wait --timeout 10m
}

Write-Host '==> Karma + приёмник вебхуков' -ForegroundColor Cyan
Invoke-Native 'helm alerting' {
    helm upgrade --install alerting (Join-Path $deploy 'charts\alerting') --namespace $ns --wait --timeout 10m
}

Write-Host '==> api' -ForegroundColor Cyan
Invoke-Native 'helm api' {
    helm upgrade --install api (Join-Path $deploy 'charts\api') --namespace $ns --wait --timeout 10m
}

Write-Host ''
Write-Host 'Готово. Дальше: .\scripts\port-forward.ps1' -ForegroundColor Green
kubectl -n $ns get pods

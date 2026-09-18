param(
    [int]$Minutes = 6,
    [double]$Fail = 0.35,
    [double]$Slow = 0.25,
    [int]$Requests = 300,
    [int]$Concurrency = 20,
    [string]$BaseUrl = 'http://localhost:8088'
)

$deadline = (Get-Date).AddMinutes($Minutes)
$burst = 0

Write-Host "нагрузка до $($deadline.ToString('HH:mm:ss')): fail=$Fail slow=$Slow" -ForegroundColor Cyan

while ((Get-Date) -lt $deadline) {
    $uri = "$BaseUrl/load?requests=$Requests&concurrency=$Concurrency&fail=$Fail&slow=$Slow"
    try {
        Invoke-RestMethod -Uri $uri -TimeoutSec 15 | Out-Null
        $burst++
        Write-Host ("[{0}] burst {1} отправлен" -f (Get-Date).ToString('HH:mm:ss'), $burst)
    } catch {
        Write-Warning "burst не ушёл: $_"
    }
    Start-Sleep -Seconds 20
}

Write-Host "готово, бёрстов: $burst" -ForegroundColor Green

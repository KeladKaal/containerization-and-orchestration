Get-Job -Name 'pf-*' -ErrorAction SilentlyContinue | Stop-Job -PassThru | Remove-Job
kind delete cluster --name obs

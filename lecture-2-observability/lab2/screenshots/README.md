# Скриншоты

| Файл | Что на нём |
|---|---|
| `prometheus.png` | метрики приложения на `/actuator/prometheus`: счётчик запросов, `app_errors_total`, бакеты гистограммы |
| `grafana-datasources.png` | источники данных в Grafana: Prometheus, Loki, Jaeger |
| `red-dashboard.png` | дашборд «API — RED + logs» без нагрузки |
| `grafana-on-highload.png` | он же под нагрузкой: RPS ~7, доля 5xx ~35%, p95 2.69 с, внизу логи с `trace_id` |
| `jaeger-slow-waterflow.png` | трейс `/slow`, вложенный span `slow-op` занимает почти всё время |
| `jaeger-fail-waterflow.png` | трейс `/fail` с красным span |
| `alerts-prometheus.png` | группа правил `api.slo`, `ApiLatencyP95High` в firing |
| `alerts-alertmanager.png` | алерты дошли до Alertmanager |
| `alerts-karma.png` | те же алерты в Karma |
| `bonus-skript-checks.png` | прогон `scripts/verify.ps1` |

Логи в Grafana отдельным файлом не снимал — они видны на дашборде (`red-dashboard.png`,
`grafana-on-highload.png`), нижняя панель.

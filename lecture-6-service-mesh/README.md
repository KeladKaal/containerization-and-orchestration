# Лекция 6 — Service mesh и продвинутый трафик

Зачем нужен mesh, как он устроен и чего он стоит. Data plane против control plane, sidecar-модель на Envoy и сдвиг в сторону sidecarless и eBPF.

## Блоки

1. **Зачем mesh** — L7-задачи, вынесенные из кода приложения; data plane против control plane; Envoy как sidecar; перехват трафика.
2. **Что даёт mesh** — mTLS и identity нагрузки; управление трафиком (canary, retry, timeout, circuit breaking); наблюдаемость.
3. **Цена и эволюция** — накладные расходы sidecar-модели; ambient / sidecarless; eBPF-подход; когда mesh не нужен вовсе.

> Конспект-самоучитель и лаба появятся позже.

# Лекция 3 — Control plane Kubernetes

Поднимаемся в оркестрацию. Декларативная модель и reconciliation loop как центральная идея, компоненты control plane и полный путь запроса от `kubectl apply` до пода на ноде.

## Блоки

1. **Декларативная модель** — желаемое против фактического состояния; reconciliation loop как центральная идея; объекты, спеки, статусы.
2. **Компоненты control plane** — API-сервер, etcd (raft, watch, resourceVersion, оптимистичная блокировка), планировщик, controller-manager.
3. **Путь запроса** — от `kubectl apply` до пода на ноде; admission; kubelet и его цикл сверки; CRI/CNI/CSI на ноде.

> Конспект-самоучитель и лаба появятся позже.

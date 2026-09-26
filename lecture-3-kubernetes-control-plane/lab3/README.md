# Лаба 3 — Собери платформу для shop

## Часть 0: Сервис сам себя не напишет, а нейронка напишет :)

Сгенерировал сервис на Go

### Конфигурация

| Variable | Service | Default    | Meaning                                        |
| --- |---|------------|------------------------------------------------|
| `DATABASE_URL` | api worker | обязателен | PostgreSQL connection string                   |
| `HTTP_ADDR` | api worker  | `:8080`    | Health/API ожидает запрос              |
| `HEALTH_FAIL` | api worker  | `false`    | Возвращает HTTP 503 от `/health` если HEALTH_FAIL=true     |
| `POLL_INTERVAL` | worker | `1s`       | Как часто проверять наличие отложенных заказов |

## API

Создать заказ:

```console
curl -i -X POST http://localhost:8080/order \
  -H 'Content-Type: application/json' \
  -d '{"sku":"book-1","quantity":2}'
```

Список заказов:

```console
curl http://localhost:8080/orders
```

Проверка health:

```console
curl -i http://localhost:8080/health
```

## Стенд

| Компонент | Конфигурация |
| --- | --- |
| Хост | macOS 26.3.1, Apple Silicon, Docker Desktop 27.4.0 |
| Кластер | k3d 5.9.0, K3s/Kubernetes 1.35.5, 3 ноды (1 server + 2 agents) |
| Helm | 4.3.0 |
| Сервисы | Go 1.22.6, `net/http`, pgx 5.7.2 |

## Часть 1 — Ограждения на кластер

## Выбор движка политик
Между OPA/Gatekeeper и Kyverno выбрал Kyverno. В лабе рассматривается только начало работы с кубером, на этом этапе его будет достаточно.
Для использования Gatekeeper нужно было бы добавлять доп. слой, что в контексте лабы не оправдано.

#### Описали правила для кластера
- Обязательные CPU/memory requests и limits.
requests нужны scheduler`у для размещения, limits ограничивают потребление на ноде.

- Только доверенный registry образов.
Запрещает запуск артефактов из неизвестных источников.

- Обязательные стандартные labels.
Требуем app.kubernetes.io/name, app.kubernetes.io/part-of и app.kubernetes.io/managed-by.

- Запрет privileged-контейнеров.
Не разрешаем securityContext.privileged: true, который почти снимает изоляцию от хоста.

- Запрет host namespaces.
Не разрешаем hostPID, hostIPC и hostNetwork, чтобы Pod не разделял пространства имён с нодой.

- Запуск не от root.
Требуем runAsNonRoot: true, чтобы процесс не стартовал с UID 0.

- Запрет повышения привилегий.
Требуем allowPrivilegeEscalation: false, чтобы процесс не получил больше прав после запуска.

Запустили кластер, добавили и протестировали политики

![polices.png](polices.png)
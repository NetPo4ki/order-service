# order-service

Высоконагруженный сервис заказов: e-commerce order-service
с гонкой за последний товар на складе. Go + PostgreSQL + Redis, Clean
Architecture / DDD.

Что демонстрирует код:

- защита от double-booking через транзакции + advisory locks в Postgres
- идемпотентность API (`Idempotency-Key`)
- rate limiting (Redis, распределённый) и graceful shutdown
- HTTP и gRPC как два равноправных транспорта над одним usecase-слоем
- разделение на domain / usecase / repository / delivery-слои

## Стек

Go 1.25 · PostgreSQL 16 · Redis 7 · pgx/v5 · net/http (`http.ServeMux`) · gRPC · Docker

## Быстрый старт

Нужен только Docker.

```bash
docker compose up --build
```

Поднимет Postgres, Redis, накатит миграции (сервис `migrate`) и запустит API:
HTTP на `http://localhost:8080`, gRPC на `localhost:9090`.

Добавить тестовый товар и создать заказ:

```bash
docker compose exec postgres psql -U orders -d orders \
  -c "INSERT INTO products (name, price, stock) VALUES ('Chair', 1999, 1);"

curl -X POST http://localhost:8080/orders \
  -H "Idempotency-Key: demo-1" \
  -d '{"user_id":1,"product_id":1,"quantity":1}'
```

Остановить и удалить всё (включая volume с данными):

```bash
docker compose down -v
```

## Локальный запуск без Docker

Нужны Go 1.25+, доступные Postgres и Redis (можно поднять их одних через
`docker compose up postgres redis`), утилита [golang-migrate](https://github.com/golang-migrate/migrate) для миграций.

```bash
migrate -path migrations \
  -database "postgres://orders:orders@localhost:5432/orders?sslmode=disable" up

DATABASE_URL="postgres://orders:orders@localhost:5432/orders?sslmode=disable" \
REDIS_ADDR="localhost:6379" \
go run ./cmd/server
```

### Переменные окружения

| Переменная             | Обязательна | По умолчанию | Описание                                  |
|-------------------------|:-----------:|--------------|--------------------------------------------|
| `DATABASE_URL`          | да          | —            | DSN подключения к Postgres                 |
| `REDIS_ADDR`            | нет         | `localhost:6379` | адрес Redis для rate limiting          |
| `HTTP_ADDR`             | нет         | `:8080`      | адрес, на котором слушает HTTP-сервер      |
| `GRPC_ADDR`             | нет         | `:9090`      | адрес, на котором слушает gRPC-сервер      |
| `ADMIN_ADDR`            | нет         | `:9100`      | адрес служебного сервера (`/healthz`, `/readyz`, `/metrics`) |
| `RATE_LIMIT_REQUESTS`   | нет         | `20`         | лимит запросов на клиента за окно (HTTP)   |
| `RATE_LIMIT_WINDOW`     | нет         | `10s`        | длительность окна лимита (формат `time.ParseDuration`) |

### Конфигурация и секреты

Все настройки — через переменные окружения (`internal/config`), без файлов
конфига. Для чувствительных значений (сейчас — `DATABASE_URL`) поддержана
конвенция `<KEY>_FILE`: если задана `DATABASE_URL_FILE=/path/to/file`,
значение читается из этого файла, а сама переменная `DATABASE_URL`
игнорируется. Это тот же механизм, которым:

- Kubernetes монтирует `Secret` как файл в под;
- HashiCorp Vault Agent Injector кладёт секрет, полученный из Vault, в файл
  внутри пода (обычно `/vault/secrets/...`) — приложению для интеграции с
  Vault не нужно ничего знать про Vault API, sidecar сам поддерживает файл
  в актуальном состоянии.

Сервис в этом репозитории Vault не разворачивает (это была бы отдельная
инфраструктура ради pet-проекта) — но код уже готов принять секрет именно
так, если `DATABASE_URL_FILE` в окружении окажется настоящим path'ом,
подложенным Vault Agent Injector'ом или K8s Secret volume.

## API

### `POST /orders` — создать заказ

```bash
curl -X POST http://localhost:8080/orders \
  -H "Idempotency-Key: <uuid клиента>" \
  -d '{"user_id":1,"product_id":1,"quantity":1}'
```

`Idempotency-Key` необязателен, но рекомендован для мутирующих запросов:
повтор с тем же ключом и тем же телом вернёт байт-в-байт тот же ответ, не
выполняя списание остатка повторно; тот же ключ с другим телом — `422`.

Возможные ответы: `201` (создан), `400` (невалидные данные), `404` (нет
такого товара), `409` (нет в наличии), `422` (Idempotency-Key переиспользован
с другим телом), `429` (превышен rate limit, см. заголовок `Retry-After`).

### `GET /orders/{id}` — получить заказ

```bash
curl http://localhost:8080/orders/<id>
```

`200` или `404`.

### gRPC (`orderv1.OrderService`)

Тот же `CreateOrder`/`GetOrder` поверх тех же usecase, что и HTTP — см.
[proto/orderv1/order.proto](proto/orderv1/order.proto). Сервер регистрирует
[reflection](https://github.com/grpc/grpc/blob/master/doc/server-reflection.md),
поэтому `grpcurl` работает без локальной копии `.proto`:

```bash
grpcurl -plaintext -d '{"user_id":1,"product_id":1,"quantity":1}' \
  localhost:9090 orderv1.OrderService/CreateOrder

grpcurl -plaintext -d '{"id":"<uuid>"}' \
  localhost:9090 orderv1.OrderService/GetOrder
```

Ошибки маппятся в коды gRPC так же, как в HTTP-статусы: `FailedPrecondition`
(нет в наличии, ~409), `InvalidArgument` (~400), `NotFound` (~404), `Internal`
(~500). Idempotency-Key и rate limiting сейчас реализованы только на HTTP-
транспорте (это HTTP-специфичные cross-cutting concerns — см. `internal/delivery/http/middleware`);
для gRPC их роль сыграли бы interceptor'ы, если понадобятся.

## Тесты

```bash
# юнит-тесты (домен, usecase-логика, rate limiter на miniredis) — без Docker
go test ./... -race

# + интеграционный тест защиты от double-booking на настоящем Postgres
# (поднимает контейнер через testcontainers-go, нужен Docker)
go test -tags=integration ./... -race
```

`TestCreateOrderUseCase_LastItemRace` (`internal/repository/postgres/integration_test.go`)
запускает 30 параллельных горутин на товар с `stock=1` и проверяет, что
успешным становится ровно один заказ.

## DevOps / production packaging

Помимо `docker-compose.yml` для локальной разработки, в репозитории есть
полная "прод-обвязка" — то, что превращает pet-проект в что-то деплоящееся
по-настоящему:

- **health/readiness**: `/healthz` (liveness, не проверяет зависимости —
  иначе временный сбой Postgres/Redis вызвал бы каскадные рестарты пода) и
  `/readyz` (readiness — 200 только пока `internal/observability.Readiness`
  выставлен в `true`; сбрасывается в `false` первым шагом graceful
  shutdown, до начала `Shutdown`/`GracefulStop`, чтобы Kubernetes успел
  убрать под из `Service` до того, как соединения реально начнут рваться).
  Для gRPC — тот же принцип через стандартный `grpc.health.v1.Health`.
  Всё на отдельном порту (`ADMIN_ADDR`, по умолчанию `:9100`), не смешано с
  бизнес-API.
- **метрики**: `/metrics` (Prometheus) на том же admin-порту — HTTP
  (`http_requests_total`, `http_request_duration_seconds` с лейблом
  статического route, не сырого URL) и gRPC (`grpc_server_requests_total`,
  `grpc_server_request_duration_seconds`) плюс стандартные Go/process
  метрики из `client_golang`.
- **структурные логи**: `log/slog` с JSON-хендлером — читаемо и для `grep`,
  и для Loki/Promtail.

### Мониторинг локально (Prometheus + Grafana + Loki)

```bash
docker compose -f docker-compose.yml -f docker-compose.observability.yml up --build
```

Поднимает вдобавок к основному стеку Prometheus (скрейпит `app:9100/metrics`),
Loki + Promtail (собирает логи всех контейнеров через Docker socket) и
Grafana с уже прогруженным дашбордом ("order-service — RED metrics": request
rate/error rate/p95 latency по HTTP и gRPC, goroutines, RSS, последние
логи). Grafana — `http://localhost:3000` (анонимный доступ включён, это
demo-конфигурация, не для реального интернета). Prometheus UI —
`http://localhost:9091`.

### Kubernetes (Helm)

```bash
helm lint ./charts/order-service
helm template order-service ./charts/order-service | less   # что сгенерируется
helm install order-service ./charts/order-service --namespace order-service --create-namespace
```

По умолчанию (`values.yaml`) чарт разворачивает не только сам сервис, но и
Postgres/Redis как `StatefulSet` внутри кластера — специально, чтобы на
локальном kind/minikube `helm install` давал такой же цельный стенд, как
`docker compose up`. `values-prod.yaml` — пример реального прод-оверрайда:
свои Postgres/Redis выключены (`postgres.enabled: false`), вместо них —
managed-инстансы через `existingSecretName`, включён `Ingress` и
`ServiceMonitor` (требует Prometheus Operator в кластере):

```bash
helm upgrade --install order-service ./charts/order-service \
  -f charts/order-service/values-prod.yaml \
  --set image.tag=$GITHUB_SHA \
  --namespace order-service --create-namespace
```

Deployment использует `preStop`-хук (`sleep 5`) в паре с `readyz` — время
для `Endpoints controller`'а убрать под из ротации до того, как приложение
начнёт закрывать соединения; без этой пары часть трафика неизбежно попадала
бы в под, уже получивший `SIGTERM`.

### CI/CD

`.github/workflows/ci-cd.yml`: `lint` (`golangci-lint`, `gofmt`) → `test`
(юнит + `-tags=integration` на testcontainers) → `build` (сборка образа,
push в GHCR на `main`) → `deploy` (`helm upgrade --install` в кластер, только
на `main`, за ручным approval-гейтом через GitHub Environment `production`
— нужен секрет `KUBE_CONFIG_DATA`, которого в репозитории, разумеется, нет).

## Структура проекта

```
cmd/server/            composition root (main.go): DI, graceful shutdown
internal/
  config/              загрузка конфигурации из окружения (+ конвенция *_FILE)
  observability/        healthz/readyz/metrics, gRPC health, Prometheus middleware
  domain/              бизнес-инварианты, без зависимости от инфраструктуры
    order/              агрегат Order (конечный автомат статусов)
    product/             агрегат Product (инвариант остатка)
  usecase/              сценарии приложения, порты (интерфейсы) к инфраструктуре
  repository/postgres/  адаптеры портов usecase поверх pgx/v5
  idempotency/          порт хранилища идемпотентных ответов
  infra/
    postgres/            пул соединений
    redis/                клиент Redis
  delivery/
    http/                HTTP-обработчики, роутинг
      middleware/          idempotency, rate limiting
      response/            общие JSON-хелперы
    grpc/                gRPC-сервер (orderv1.OrderServiceServer)
      orderv1/             сгенерированный код (protoc-gen-go/-go-grpc)
proto/orderv1/         исходник order.proto
migrations/            SQL-миграции (golang-migrate)
charts/order-service/  Helm-чарт (Deployment/Service/HPA/PDB/... + опциональные
                       Postgres/Redis StatefulSet для локального кластера)
observability/         конфиги Prometheus/Loki/Promtail/Grafana для локального стека
.github/workflows/     CI/CD: lint -> test -> build -> deploy
Dockerfile             multi-stage сборка, non-root user
docker-compose.yml     Postgres + Redis + migrate + app одной командой
docker-compose.observability.yml  оверлей: + Prometheus/Grafana/Loki/Promtail
```

Зависимость слоёв направлена внутрь: `delivery`/`repository` зависят от
`usecase`, `usecase` — только от `domain`. `domain` не знает ни о Postgres,
ни о HTTP.

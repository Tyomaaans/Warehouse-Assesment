# Task 2A — backend foundation audit and completion

Date: 2026-10-22. Scope: audit preserved source, complete infrastructure/server foundation, verify it, and stop. No commit/push or next business feature is authorized.

## Audit before source changes

Reviewed AGENTS, PRD, technical spec, OpenAPI, ERD, data flow, RBAC, testing strategy, convention audit, workflow and Task 1B report, plus the original assessment. The committed documentation baseline is `b4da1a8` (following `32165d5` and `295cf3b`). All 21 backend files were untracked at the start. Their hashes were recorded in `/tmp/wh-task2a-before.json` before changes.

| Existing file | Initial assessment |
|---|---|
| `backend/go.mod` | Partial: dependencies declared, no RabbitMQ module; unused future-auth dependencies. |
| `backend/go.sum` | Incomplete: only module metadata hashes. Baseline `go test -mod=readonly ./...` failed on missing package checksums. |
| `backend/pkg/request.go` | Reusable codec, strict binding, validator tags, public messages and snake_case pagination. Binding uses Unicode TrimSpace rather than JSON whitespace; formatting incomplete. |
| `backend/pkg/error.go` | Reusable clone-style nested envelope/SQLSTATE translation. Network connection failures need unavailable mapping; formatting incomplete. |
| `backend/pkg/identity.go` | Reusable role/context helpers; no authentication middleware establishes trusted identity yet. |
| `backend/internal/infrastructure/postgres/postgres.go` | Partial: GORM pool exists; automatic open/ping can ignore startup context, and ping failure does not close the pool. |
| `backend/internal/infrastructure/redis/redis.go` | Reusable context-bound startup/pool; needs safe structured client logging and bounded pool waiting. |
| `backend/migrations/migrate.go` | Reusable embedded numbered SQL, SHA-256 checksums, UUID bookkeeping and transaction advisory lock. Not previously executed. |
| `backend/migrations/001_receiving.sql` | Existing schema matches primary tables/statuses, partial active-TRX uniqueness, ledger uniqueness, actor/time pairs and notification composite uniqueness. Requires real PostgreSQL validation; item UPDATE guard checks only the new parent and can miss an immutable old parent. |
| `backend/internal/products/domain/product.go` | Present entities/repository interface; internal UUID type and field names align with schema. |
| `backend/internal/products/dto.go` | Present create/list DTOs; Base64URL IDs are mapped at response boundary; request tags match current product contract. |
| `backend/internal/products/storage.go` | Present UUID PK and explicit products table name. |
| `backend/internal/products/mapper.go` | Reusable binary UUID-to-Base64URL mapping. |
| `backend/internal/products/repository.go` | Present GORM transaction/parameterized queries and deterministic pagination. Not wired or behaviorally verified. |
| `backend/internal/products/service.go` | Present role/struct validation. Pagination assumes valid handler inputs; future service callers must preserve that precondition. No auth wiring exists. |
| `backend/internal/products/handler.go` | Present strict binding/standard responses; deliberately not registered in foundation server. |
| `backend/internal/receiving/domain/receiving.go` | Partial: entities, statuses, transaction callback/repository interfaces; no receiving orchestration service. |
| `backend/internal/receiving/dto.go` | Present locked actor/time fields, positive integer tags and correction/review DTOs; not a complete workflow. |
| `backend/internal/receiving/storage.go` | Present explicit tables and nullable metadata; migration remains schema authority. |
| `backend/internal/receiving/mapper.go` | Present public IDs, remaining quantities and history metadata mappings. |
| `backend/internal/receiving/repository.go` | Partial persistence, parent-first locking, source-linked ledger/audit/outbox helpers. State/cap/duplicate-line validation and atomic approval orchestration are missing service responsibilities. Transaction list metadata/items use separate reads; later receiving work should ensure a consistent read snapshot. |

Missing foundation: executable entry point, environment validation, Gin router, zerolog request/panic logging, health/readiness, RabbitMQ setup, graceful shutdown, Dockerfile, Compose/Nginx and startup instructions. JWT/session authentication, receiving services, inventory/audit/notification HTTP features, outbox delivery and frontend remain outside Task 2A.

## Reference comparison and implementation boundaries

- `/home/tyomaans/Projects/auth-session`: injected constructors, feature ownership, shared infrastructure/config/router, binary UUID Base64URL algorithm and HTTP lifecycle. Preserve its architecture, with the approved strict binding and clone envelope adaptations.
- `/home/tyomaans/Projects/clone-mini-erp/backend`: context/pool setup, strict JSON, SQLSTATE translation, zerolog redaction, numbered embedded migrations with checksums/advisory locking, and staged non-root Docker build.
- `/home/tyomaans/Projects/email-job/internal/infrastructures/rabbitmq/rabbitmq.go`: AMQP 0-9-1 connection/channel lifecycle using amqp091-go. Reuse only connection lifecycle, not email queues, retries or business workers.
- Existing standard-library test style follows the inspected `payment-gateway/internal/payment-gateway/cart/service_test.go`. Keep automated additions limited to critical T07 request/error/ID contracts; use infrastructure smoke checks for lifecycle/migrations.

The locked API and PRD remain authoritative. Only `/api/v1/health` and `/api/v1/ready` will be registered; unimplemented business routes will not expose unauthenticated handlers. PostgreSQL/Redis determine API readiness. RabbitMQ connection availability is observed/retried independently so notification infrastructure cannot gate receiving availability; no publisher/consumer is implemented here.

## Execution evidence

Baseline checks: `gofmt -l backend` found 17 unformatted files; `go test -mod=readonly ./...` failed because package checksums were missing. No baseline application test passed.

### Reuse and changed-file inventory

All 21 existing files were retained; none was deleted. `backend/internal/products/mapper.go` is byte-for-byte unchanged. Correct existing feature interfaces, DTOs, models, mappers, repositories, codec and migration logic were reused.

Existing files with functional changes:

```text
backend/go.mod
backend/go.sum
backend/internal/infrastructure/postgres/postgres.go
backend/internal/infrastructure/redis/redis.go
backend/migrations/001_receiving.sql
backend/migrations/migrate.go
backend/pkg/error.go
backend/pkg/request.go
```

Existing files changed by gofmt only; business behavior was not expanded:

```text
backend/pkg/identity.go
backend/internal/products/domain/product.go
backend/internal/products/dto.go
backend/internal/products/handler.go
backend/internal/products/repository.go
backend/internal/products/service.go
backend/internal/products/storage.go
backend/internal/receiving/domain/receiving.go
backend/internal/receiving/dto.go
backend/internal/receiving/mapper.go
backend/internal/receiving/repository.go
backend/internal/receiving/storage.go
```

New files:

```text
.env.example
compose.yml
deploy/nginx.conf
backend/.dockerignore
backend/Dockerfile
backend/main.go
backend/internal/config/config.go
backend/internal/infrastructure/rabbitmq/rabbitmq.go
backend/internal/middleware/http.go
backend/internal/router/router.go
backend/pkg/request_test.go
docs/task-2a-report.md
```

Existing documentation updated: `README.md` (foundation startup/status/evidence) and `docs/ai-workflow.md` (Task 2A authorization, decisions and results). PRD, technical spec, OpenAPI, ERD, data-flow, RBAC, testing strategy and convention audit were not changed. `.gitignore` and the historical Task 1B report were also left unchanged.

### Implemented repairs and decisions

- Completed module sums/transitive requirements and added amqp091-go 1.14.0 from the inspected email-job reference. Removed unused direct JWT/crypto requirements; JWT/session code remains a later feature, and required indirect crypto dependencies remain managed by Go.
- Added validated environment configuration and injected Gin wiring. Only the two public infrastructure routes are registered; no product/receiving handler is exposed without authentication. Unmatched routes and recovered panics use the locked JSON envelope.
- Added zerolog request/lifecycle events that do not log request bodies, query strings, credentials, token values or panic contents. Redis internal logs use the inspected redaction adapter. Connection/configuration failures use sanitized messages.
- Fixed PostgreSQL initialization to disable GORM's implicit unbounded ping, use the supplied context, and close the pool on ping failure. Bounded Redis pool waiting and mapped database/network unavailability to the existing public 503 convention.
- Added bounded AMQP connection/channel setup, independent retry/reconnection and shutdown. No exchange, queue, publisher or consumer is introduced. Readiness checks PostgreSQL, Redis and schema checksums; RabbitMQ availability remains ancillary to API readiness, preserving BR-020.
- Reused the migration runner and added read-only checksum checks before serving/during readiness. Fixed the existing SQL item guard to check the old parent before UPDATE, preventing reassignment away from an immutable approved/resolved TRX. This repair was made before the first Task 2A migration execution; subsequent schema changes must use a new numbered migration rather than modify an applied checksum.
- Restricted trimming to JSON whitespace; non-JSON Unicode whitespace is now rejected by strict binding. Added only T07 foundation tests for UUID encoding/decoding, request/field errors, size limits and public error statuses/redaction. These do not establish completion of T07's future business-endpoint checks.
- Added a non-root multi-stage backend image, explicit migration service and Compose PostgreSQL/Redis/RabbitMQ/Nginx services with persistent volumes. An initial Compose smoke run found the fixed Nginx IP could collide with dynamic allocation; `WAREHOUSE_IP_RANGE` now separates the dynamic pool from `NGINX_IP`. The rerun succeeded.
- Nginx forwards API paths unchanged, streams requests to Gin's 1 MiB binder and converts upstream failures to the locked nested 503 JSON envelope. Only loopback Nginx ingress is published; database/cache/broker ports remain internal. TLS/public deployment is outside this task.
- SIGTERM drains HTTP requests, then closes PostgreSQL/Redis and the AMQP lifecycle with bounded waits. No AutoMigrate or automatic business-module registration was added.

### Exact verification commands and results

Go commands ran from `/home/tyomaans/Projects/wh-assesment/backend`. Writable agent caches were selected with:

```sh
export GOTOOLCHAIN=local
export GOPATH=/tmp/wh-go-path
export GOMODCACHE=/tmp/wh-go-mod
export GOCACHE=/tmp/wh-go-build
go mod tidy
go test ./...
go build ./...
go vet ./...
```

All completed with exit code **0** after dependency repair. `go test ./...` passed the three T07 foundation test functions in `warehouse/pkg`, including 13 request subcases; the other packages compiled and reported `[no test files]`. No receiving, approval, inventory or notification workflow test is claimed.

From the repository root:

| Command | Actual result |
|---|---|
| `gofmt -w backend` followed by `gofmt -l backend` | Formatting completed; final listing empty. |
| `docker compose --env-file .env.example -p wh-task2a config --quiet` | Exit 0; Compose configuration valid. |
| `docker compose --env-file .env.example -p wh-task2a build api` | Image built successfully with Go 1.26.4; first sandbox attempt could not write Docker build metadata, then approved execution succeeded. |
| `docker compose --env-file .env.example -p wh-task2a up -d postgres redis rabbitmq` | PostgreSQL 16, Redis 7 and RabbitMQ 4 started and became healthy. |
| `HTTP_PORT=18080 docker compose --env-file .env.example -p wh-task2a up -d --no-build --wait` | Initial run applied migration and started healthy API but detected Nginx address collision; rerun after the IPAM fix passed with all long-running services healthy and migrate exit 0. Named volumes were preserved while recreating the task's network/containers. |
| `python3 /tmp/wh-task2a-smoke.py` | **11 infrastructure/schema smoke checks passed**, detailed below. Temporary script and `/tmp/wh-task2a-smoke-results.json` retain the exact assertions/results for inspection. |
| `PYTHONPATH=/tmp/wh-contract-validation python3 -m openapi_spec_validator docs/openapi.yml` | **OK**, unchanged OpenAPI contract remains valid. |
| `git diff --check` | Exit 0; no whitespace errors. |
| `HTTP_PORT=18080 docker compose --env-file .env.example -p wh-task2a stop` | Stops this task's test stack; preserves named volumes and container evidence. Historical Task 1B test containers/data were not changed. |

The temporary smoke script uses `docker compose exec -T postgres psql -X -A -t -v ON_ERROR_STOP=1 -U warehouse -d warehouse` for schema checks and `docker compose run --rm --no-deps migrate` for reruns/checksum validation. HTTP checks go through Nginx at `http://127.0.0.1:18080/api/v1`; bodies are validated with the locked OpenAPI response schemas.

The 11 passed checks:

1. Health/readiness match their OpenAPI bodies; `/products` returns the standard 404 because business routes are unregistered.
2. Migration created 12 UUID-keyed tables (including bookkeeping), the partial active-TRX index and composite notification uniqueness.
3. Two concurrent migration reruns succeed with one bookkeeping row.
4. A separate PostgreSQL transaction holds advisory lock 88620; the migration runner is observed waiting for it, then succeeds after release.
5. A deliberately changed checksum is rejected by migration and readiness; restoring it recovers readiness. Process health stays 200.
6. SQL guard rejects moving an approved item to a pending parent. The fixture is rolled back; products/ledger remain empty.
7. Stopping RabbitMQ leaves API readiness at 200.
8. Restarting RabbitMQ produces a new connection event without restarting the API.
9. Stopping Redis returns readiness 503 with the exact nested error shape, while health stays 200.
10. Restarting Redis recovers readiness.
11. SIGTERM drains HTTP and exits the API with code 0; Nginx then returns the locked 503 envelope for the unavailable upstream.

## Remaining work and readiness for review

No Task 2A foundation verification is blocked. The foundation is ready for a **separate, scoped Git commit after explicit approval**; nothing was staged, committed or pushed by this task. The original 21 files remain in place, with formatting-only business changes clearly separated above.

The application is not feature-complete. JWT/cookie sessions and backend RBAC middleware, receiving/correction/review/approval service orchestration, exactly-once inventory workflow tests T01–T06, business-endpoint completion of T07, notification outbox delivery/consumer, frontend and public/TLS deployment remain separate work. Existing unregistered repositories are building blocks, not proof of those behaviors. The service pagination precondition and receiving list snapshot risk recorded in the initial audit remain for their respective future feature reviews.

Stop after Task 2A and await approval before the next business feature or any Git mutation.

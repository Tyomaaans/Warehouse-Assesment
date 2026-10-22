# Task 2B — Authentication, Session Management & RBAC

Date: 2026-10-22. Baseline: clean working tree at `0a1dc8a`. Status: **implemented and verified; ready for review and a separate commit after approval**. No staging, commit or push was performed.

## Scope and references

Read `AGENTS.md`, the PRD, technical specification, locked OpenAPI, RBAC, conventions audit, testing plan and AI workflow before implementation; inspected the existing foundation. Reused actual `/home/tyomaans/Projects/auth-session/internal/auth-session/{domain,user,session,token}`, its auth middleware, configuration, bcrypt provisioning and cookie handlers. Unit tests follow the simple repository-stub and `testing` assertions in `/home/tyomaans/Projects/payment-gateway/internal/payment-gateway/cart/service_test.go`.

The only added production routes are POST `/api/v1/auth/login`, `/api/v1/auth/refresh` and `/api/v1/auth/logout`. Existing GET health/readiness routes remain available. The contract defines no identity endpoint. Role-guard routes under `/test` exist only inside integration tests. Product, receiving, approval, inventory, notification processing and frontend workflows were not implemented or registered.

## Files changed

Created (17 files):

- `backend/internal/auth/domain/user.go` — user entity/repository interface and valid roles.
- `backend/internal/auth/domain/session.go` — internal session, refresh payload and token-pair entities.
- `backend/internal/auth/user/dto.go` — exact login/user/refresh DTOs; optional boolean rejects explicit JSON null.
- `backend/internal/auth/user/storage.go` — existing PostgreSQL `users` schema mapping.
- `backend/internal/auth/user/mapper.go` — storage/entity/public Base64URL ID mapping.
- `backend/internal/auth/user/repository.go` — GORM context-bound identity queries and existing error translation.
- `backend/internal/auth/user/service.go` — validation, bcrypt credential verification and auth orchestration.
- `backend/internal/auth/user/handler.go` — strict binding, success/errors, cookies and empty 204 logout.
- `backend/internal/auth/user/provision.go` — explicit role-account provisioning in a GORM transaction.
- `backend/internal/auth/session/service.go` — Redis JSON/session/set storage and atomic create/rotate/revoke scripts.
- `backend/internal/auth/token/service.go` — HS256 JWTs, hashed refresh generation, rotation lock and live identity checks.
- `backend/internal/middleware/auth.go` — authentication, trusted actor context and separate role guards.
- `backend/internal/auth/user/service_test.go` — credential failures, dependency failures, bcrypt truncation and service identity.
- `backend/internal/auth/token/service_test.go` — expiry, signature/algorithm, required claims, UUIDs, roles and lifetime validation.
- `backend/internal/auth/auth_integration_test.go` — actual PostgreSQL/Redis API, session, concurrency and RBAC tests.
- `backend/internal/config/auth_test.go` — mandatory signing secret and refresh-duration defaults/validation.
- `docs/task-2b-report.md` — this report.

Modified (9 files):

- `backend/go.mod`, `backend/go.sum` — JWT v5.3.1 and direct bcrypt dependency; retained existing versions otherwise.
- `backend/internal/config/config.go` — signing secret and configurable whole-second refresh TTLs.
- `backend/internal/router/router.go` — explicit auth handler/middleware injection and the three routes.
- `backend/main.go` — auth dependency wiring and explicit `provision-users` command.
- `.env.example`, `compose.yml` — JWT secret/refresh expiry settings; provisioning environment examples.
- `README.md` — current endpoint scope, setup/provisioning and integration-test instructions.
- `docs/ai-workflow.md` — Task 2B authorization, decisions and executed verification.

Correct foundation work reused without changes: `backend/pkg/{request,error,identity}.go`, `backend/pkg/request_test.go`, PostgreSQL/Redis/RabbitMQ connection packages, structured request logging, migrations and the Dockerfile. Preserved product/receiving files were not changed. No SQL migration, PRD, OpenAPI, ERD, business flow, RBAC or convention-audit contract was changed.

## Implementation decisions

- Password verification uses the reference's bcrypt convention and lowercase email lookup. Unknown accounts and incorrect passwords return the same generic 401 `invalid_credentials`. Database failures retain their dependency/error classification. Passwords over bcrypt's 72-byte limit cannot authenticate through truncation. No password or token appears in public errors or added logs.
- Access tokens use HS256 with internal textual UUID v4 `sub`/`sid`, a supported role, required `iat`/`exp`, and exactly 900 seconds between issue and expiry. Parsing validates the signature, allowed algorithm, expiry and issued-at time. REST user/session-cookie IDs reuse the foundation's canonical 22-character Base64URL UUID v4 codec.
- Every protected request verifies the live Redis session and matching user/session/role, then checks that the database user still exists with the same role. Refresh performs the same identity checks. Redis errors return generic 503; absent, expired, replayed or revoked credentials return 401.
- Refresh tokens are 32 cryptographically random bytes encoded as hex; only their SHA-256 hashes are persisted. Reference keys remain `refresh:<hash>`, `session:<SHA256(textual session UUID)>` and `user_sessions:<textual user UUID>`. Internal JSON retains the reference's session/refresh identity, role, timestamps and TTL fields; heartbeat/session-management metadata and APIs are outside this task.
- Rotation uses the reference's five-second `SET NX` lock, random ownership value and ownership-checked Lua release. Atomic Redis scripts check ownership and current session/refresh state before replacing tokens. Revocation removes the session, its current refresh record and user-set membership atomically. A delayed refresh cannot recreate a revoked session. User-set expiry is extended without shortening the lifetime needed by another session.
- Both refresh cookies must identify the same session. Logout additionally requires a valid Bearer JWT for that user/session, and cannot revoke another user's session. A stale refresh cookie after completed rotation returns 401; the successful rotation's cookies can then be used to log out. Successful logout returns empty 204 and immediately invalidates the session's existing access JWTs.
- Cookies are host-only/HttpOnly/SameSite Strict: `refresh_token` at `/api/v1/auth`, `session_id` at `/api/v1`. Production mode sets Secure. Cookie Max-Age equals Redis/session TTL: default 1h or 168h with `remember_me`; refresh replaces both cookies in the same session. Logout clears them at identical paths/security attributes with wire `Max-Age=0`. Token responses use `Cache-Control: no-store` and never include a refresh token in JSON.
- Role guards use the verified actor context and allow only explicitly listed roles. Supervisor does not inherit Operator-only permissions. Sensitive services can reuse `pkg.RequireActor`; logout already enforces it. Future business route/service guards remain part of their own implementation tasks.
- `provision-users` reads `OPERATOR_EMAIL`/`OPERATOR_PASSWORD` and `SUPERVISOR_EMAIL`/`SUPERVISOR_PASSWORD`. It validates and hashes before inserting both accounts in one transaction. Same email/role preserves the existing password; a different role aborts. This is the locked specification's operator tooling, with no public registration or account-management API.

## Executed verification

Go 1.26.4, PostgreSQL 16 and Redis 7 were available. All commands below completed with exit code 0. Go commands ran from `/home/tyomaans/Projects/wh-assesment/backend` with these writable caches:

```sh
export GOTOOLCHAIN=local
export GOPATH=/tmp/wh-go-path
export GOMODCACHE=/tmp/wh-go-mod
export GOCACHE=/tmp/wh-go-build
```

| Command | Actual result |
|---|---|
| `go mod tidy` | Pass; existing dependency versions retained, JWT added, bcrypt dependency direct |
| `gofmt -w main.go internal/auth internal/config/config.go internal/middleware/auth.go internal/router/router.go` | Applied formatting; newly added config/unit tests formatted separately |
| `gofmt -l .` | Pass; no output |
| `go test ./...` | Pass; integration explicitly skipped when test URLs were unset |
| `go build ./...` | Pass |
| `go vet ./...` | Pass |
| `go build -o /tmp/wh-task2b-api .` | Pass; binary used in live-server smoke |
| `TEST_DATABASE_URL=postgres://warehouse:task2b-isolated-test-password@127.0.0.1:32770/warehouse_auth_test?sslmode=disable TEST_REDIS_URL=redis://127.0.0.1:32771/0 go test -count=1 ./...` | Pass; real integration executed, auth package 3.909s; token, user, config and existing shared-contract tests also passed |
| Same test URLs with `go test -count=1 -v ./...` | Pass; all 12 auth integration scenarios executed; later added truncation/config unit tests also passed in the final full-suite run above |
| Same test URLs with `go test -race -count=1 ./internal/auth/...` | Pass; auth integration 34.717s, token 1.045s, user 2.629s; no races reported |
| `python3 /tmp/wh-task2b-smoke.py` | Pass; real binary migration, idempotent provisioning, health/readiness, both roles' login/refresh/replay/logout/access revocation, missing business/identity routes and graceful SIGTERM |
| `env JWT_SECRET_KEY=task2b-compose-validation-secret-at-least-32-bytes docker compose --env-file .env.example config --quiet` | Pass |
| `PYTHONPATH=/tmp/wh-contract-validation python3 -m openapi_spec_validator docs/openapi.yml` | `docs/openapi.yml: OK` |
| `git diff --check` | Pass |

Disposable infrastructure was started with:

```sh
docker run -d --name wh-task2b-postgres --label warehouse-task=2b \
  -e POSTGRES_USER=warehouse -e POSTGRES_DB=warehouse_auth_test \
  -e POSTGRES_PASSWORD=task2b-isolated-test-password \
  -p 127.0.0.1::5432 postgres:16-alpine
docker run -d --name wh-task2b-redis --label warehouse-task=2b \
  -p 127.0.0.1::6379 redis:7-alpine redis-server --save '' --appendonly no
```

The generated host ports were 32770/32771. Credentials above belong only to these disposable test containers. Integration tests apply and verify the existing checksummed migrations in a new UUID-named schema and remove that schema afterward; Redis cleanup is scoped to generated identities/session hashes, without FLUSHDB. The smoke binary used the separate test container's default schema and runtime-generated account passwords/signing secret. The existing PostgreSQL service on host port 5432 and all earlier project containers/data were preserved. Temporary smoke code, logs and binaries live under `/tmp` and are not repository deliverables.

Integration evidence includes invalid credentials/strict JSON/413, deterministic rejection of rotation after revocation, fail-closed Redis errors, both directions of role denial, hashed storage/TTLs, refresh replay/lock contention, mismatched cookies/Bearer, empty logout/clearing cookies, expired JWT/refresh/session, changed database role, eight competing refreshes with exactly one success, and eight logout/refresh races. Existing T07 UUID/error/validation tests still pass. Migration execution/checksum validation passed both in the isolated suite and through the real CLI. No receiving/inventory correctness claim is made by these auth tests.

## Remaining limitations and review status

No unresolved specification conflict or implementation blocker was found. Production HTTPS ingress, browser/frontend integration and deployment verification remain later work; Secure cookie behavior was verified through response headers. RabbitMQ notification processing and business workflows remain outside Task 2B. Future business endpoints must apply both route guards and sensitive-service authorization; the complete T06 business-operation test is still deferred.

Provisioning is intentionally explicit and preserves existing passwords; password reset, public signup and session-management endpoints are not provided. Integration tests require both documented URLs and database schema-creation permissions; an unset environment reports an explicit skip. A Redis/network failure after an atomic mutation but before the response can leave the client without rotated cookies, requiring login again; failed requests do not bypass authentication or restore revoked state.

The isolated test containers are stopped after verification, retaining their test data. The working tree is ready for review and a separate Task 2B commit after explicit approval. No next feature, commit or push is authorized.

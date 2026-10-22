# Warehouse Receiving & Inventory Tracking System — Specification Pack
This workspace contains locked documentation, the backend foundation, Task 2B authentication/session/RBAC implementation, and preserved partial business modules. The API exposes health/readiness and the three contracted authentication endpoints. Receiving workflows, notification processing and frontend implementation remain separate tasks requiring approval.

## Documents
- `docs/prd.md` — English PRD / business source of truth.
- `docs/technical-spec.md` — architecture, database, concurrency, APIs, flows, deployment.
- `docs/erd.mmd` — Mermaid ERD.
- `docs/openapi.yml` — locked OpenAPI 1.1.0 contract, including exact envelopes, pagination and auth transport.
- `docs/conventions-audit.md` — inspected references and approved Task 1B resolutions.
- `docs/task-1b-report.md` — full change inventory, documentation validation and commit/implementation status.
- `docs/task-2a-report.md` — foundation audit, reused files, implementation scope and verification evidence.
- `docs/task-2b-report.md` — authentication/session/RBAC implementation and verification evidence.
- `docs/data-flow.mmd` — Mermaid business flow.
- `docs/rbac.md` — explicit role/action matrix.
- `docs/testing.md` — small critical test suite and evidence plan.
- `docs/ai-workflow.md` — reference inspection rules, 3 prompts, agent log.
- `AGENTS.md` — top-level agent guardrails.

## Agent handoff
1. Place docs in target repository.
2. Reference projects are under `/home/tyomaans/Projects`: `auth-session`, `clone-mini-erp`, `web-profile/web-profile-frontend`, and `search-feature-node-js`.
3. Prompt 1 inspected actual conventions; Task 1B resolved the differences. See `docs/conventions-audit.md`.
4. Validate the locked `docs/openapi.yml` and maintain cross-document consistency **before** implementation.
5. Tasks 2A and 2B cover the foundation and authentication/session/RBAC. Stop after Task 2B for review; the next business feature and any commit/push require explicit approval. Full Prompts 2 and 3 remain later implementation phases.
6. Add actual application code, migrations, Dockerfiles/Compose, `.env.example`, and a real deployment README. Record tests, limitations, and Git commits.

## Assessment
Assessment asks for PRD, technical spec, ERD, API contract, data-flow, AI workflow and >=3 meaningful prompts, critical tests, working application, reproducible Docker deployment, health endpoint, and meaningful Git history. Task 2A foundation checks are recorded in its report; those checks do not establish business-workflow correctness or completion of the application.

## Run the backend

From a clean environment with Docker and Compose, create `.env` from `.env.example`, replace the sample database and broker passwords, and set `JWT_SECRET_KEY` to a random secret of at least 32 bytes (for example, generate one with `openssl rand -hex 32`). Use URL-safe passwords, or percent-encode credentials in connection URLs. Keep `.env` untracked.

```sh
cp .env.example .env
docker compose up -d --build --wait
curl --fail http://127.0.0.1:8080/api/v1/health
curl --fail http://127.0.0.1:8080/api/v1/ready
```

Compose runs the explicit one-shot `migrate` command before starting the API. Migrations are numbered, embedded, checksum-verified and protected by a PostgreSQL transaction advisory lock. The API checks the schema before serving and during readiness; it never runs AutoMigrate. To rerun the migration command:

```sh
docker compose run --rm --no-deps migrate
docker compose stop
```

Stopping preserves named volumes. PostgreSQL, Redis, RabbitMQ and its management interface have no host-published ports. Nginx binds to localhost, forwards `/api/v1` unchanged, and has a fixed trusted proxy address configured by `NGINX_IP` inside `WAREHOUSE_SUBNET`. `WAREHOUSE_IP_RANGE` keeps dynamic service allocation away from that address. Change all three together if the subnet conflicts locally, keeping Nginx outside the dynamic range. `HTTP_PORT` defaults to 8080. TLS/public deployment remains later deployment work.

`/api/v1/health` returns `{"success":true,"message":"healthy","data":{"status":"ok"}}`. `/api/v1/ready` checks PostgreSQL, migration checksums and Redis, returning 503 with the locked nested error envelope when unavailable. RabbitMQ connects/retries independently and reports status through structured logs; no notification worker or topology is created in Task 2A. The API image runs as UID 10001 and drains HTTP requests on SIGTERM before closing infrastructure.

The registered routes are GET `/api/v1/health`, GET `/api/v1/ready`, and POST `/api/v1/auth/login`, `/api/v1/auth/refresh`, `/api/v1/auth/logout`. No identity, registration or business route is exposed. Existing product and receiving code is preserved. These checks do not substitute for T01–T06 or the final receiving smoke test.

Provision the two role accounts explicitly. Export `OPERATOR_EMAIL`, `OPERATOR_PASSWORD`, `SUPERVISOR_EMAIL` and `SUPERVISOR_PASSWORD` in your shell, then run:

```sh
docker compose run --rm --no-deps \
  -e OPERATOR_EMAIL -e OPERATOR_PASSWORD \
  -e SUPERVISOR_EMAIL -e SUPERVISOR_PASSWORD \
  api provision-users
```

This command validates both accounts and inserts bcrypt hashes in one PostgreSQL transaction. Existing accounts with the same email/role keep their passwords; a conflicting role aborts the transaction. Password provisioning accepts up to 72 bytes, matching bcrypt's limit. Nothing is seeded automatically, and there is no password-reset or public-registration endpoint.

Access JWTs expire after 900 seconds. Refresh and session cookies are HttpOnly, host-only and SameSite Strict, with paths `/api/v1/auth` and `/api/v1`. `APP_ENV=production` sets Secure cookies and requires HTTPS ingress. `SHORT_REFRESH_EXPIRY` defaults to `1h`; `DEFAULT_REFRESH_EXPIRY` defaults to `168h` for `remember_me=true`. They accept positive whole-second Go durations. Refresh rotates the token in the same session; logout requires a live Bearer token plus both cookies and returns an empty 204. Revocation immediately invalidates existing access tokens. Redis failure returns 503; current database role and live-session checks also run on protected requests.

Local verification, with Go 1.26.4 or a compatible Go 1.26 toolchain:

```sh
cd backend
gofmt -l .
go build ./...
go test ./...
go vet ./...
```

Tests cover the T07 foundation request/error/UUID contracts and focused authentication/session/RBAC cases. PostgreSQL/Redis integration tests skip explicitly unless both `TEST_DATABASE_URL` (a PostgreSQL URL) and `TEST_REDIS_URL` are set. Use disposable infrastructure: the suite creates and removes a unique database schema and uses isolated user/session IDs, without flushing Redis. To run them:

```sh
TEST_DATABASE_URL='postgres://user:password@127.0.0.1:5432/warehouse_auth_test?sslmode=disable' \
TEST_REDIS_URL='redis://127.0.0.1:6379/0' \
go test -count=1 -v ./internal/auth/...
```

The task reports list exact executed commands, including writable temporary caches used by the agent environment. RBAC integration tests use test-only guard routes; future business handlers still require their own route and service enforcement.

## Locked implementation conventions
Canonical 22-character Base64URL UUID v4 IDs; clone-style success/nested error envelopes and management pagination; strict binding plus validator/service checks; auth-session JWT and Redis/cookie revocation; versioned SQL migrations; typed React fetch with shared refresh and one retry. Exact contracts are in `docs/openapi.yml` and `docs/technical-spec.md`. No unresolved convention conflict remains after Task 1B.

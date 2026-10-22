# Warehouse Receiving & Inventory Tracking System
This workspace contains the locked specifications and the core backend: authentication/session/RBAC, products, receiving documents/transactions, Supervisor correction/review/approval, inventory and audit reads, RabbitMQ notification delivery and notification REST operations. Frontend implementation remains later work.

## Documents
- `docs/prd.md` — English PRD / business source of truth.
- `docs/technical-spec.md` — architecture, database, concurrency, APIs, flows, deployment.
- `docs/erd.mmd` — Mermaid ERD.
- `docs/openapi.yml` — locked OpenAPI 1.1.0 contract, including exact envelopes, pagination and auth transport.
- `docs/conventions-audit.md` — inspected references and approved Task 1B resolutions.
- `docs/task-1b-report.md` — full change inventory, documentation validation and commit/implementation status.
- `docs/task-2a-report.md` — foundation audit, reused files, implementation scope and verification evidence.
- `docs/task-2b-report.md` — authentication/session/RBAC implementation and verification evidence.
- `docs/task-2c-report.md` — product/receiving-document implementation, tests and Git delivery evidence.
- `docs/task-2d-report.md` — core receiving/inventory implementation, concurrency/rollback evidence and Git delivery.
- `docs/task-2e-report.md` — notification implementation, integration/recovery evidence and completed compiled-server verification.
- `docs/task-3a-report.md` — frontend foundation/authentication, npm recovery, actual browser verification and delivery evidence.
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
5. Tasks 2A–2E cover the backend and notifications. Task 2E's initial smoke-harness credentials were corrected during authorized verification; the compiled API/worker smoke and required checks passed. Commit/push uses the reviewed explicit Task 2E manifest. Stop after Task 2E; frontend implementation requires further authorization.
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

Compose runs the explicit one-shot `migrate` command before starting the API and notification worker. Migrations are numbered, embedded, checksum-verified and protected by a PostgreSQL transaction advisory lock. The API checks the schema before serving and during readiness; it never runs AutoMigrate. To rerun the migration command:

```sh
docker compose run --rm --no-deps migrate
docker compose stop
```

Stopping preserves named volumes. PostgreSQL, Redis, RabbitMQ and its management interface have no host-published ports. Nginx binds to localhost, forwards `/api/v1` unchanged, and has a fixed trusted proxy address configured by `NGINX_IP` inside `WAREHOUSE_SUBNET`. `WAREHOUSE_IP_RANGE` keeps dynamic service allocation away from that address. Change all three together if the subnet conflicts locally, keeping Nginx outside the dynamic range. `HTTP_PORT` defaults to 8080. TLS/public deployment remains later deployment work.

`/api/v1/health` returns `{"success":true,"message":"healthy","data":{"status":"ok"}}`. `/api/v1/ready` checks PostgreSQL, migration checksums and Redis, returning 503 with the locked nested error envelope when unavailable. RabbitMQ connects/retries independently and reports status through structured logs. The API image runs as UID 10001 and drains HTTP requests on SIGTERM before closing infrastructure. The notification worker is a separate command/process; broker availability does not gate receiving commits or API readiness.

Health/readiness and POST `/api/v1/auth/login`, `/api/v1/auth/refresh`, `/api/v1/auth/logout` remain registered. Task 2C also exposes:

| Method/path | Permission | Behavior |
|---|---|---|
| POST `/api/v1/products` | Supervisor | Create a unique SKU/product; no inventory writes |
| GET `/api/v1/products` | Either role | Paginated products; literal, case-insensitive SKU/name `search` |
| POST `/api/v1/receivings` | Operator | Create an `INCOMPLETE` RCV with a unique ORD reference and expected SKU quantities |
| GET `/api/v1/receivings` | Either role | Paginated summaries, optional `INCOMPLETE`/`COMPLETED` status filter |
| GET `/api/v1/receivings/{id}` | Either role | Expected, approved and remaining quantities per SKU |

Use the access token as `Authorization: Bearer <token>`. List defaults are `page=1`, `limit=10`; maxima are 10000 and 100. Responses use the locked envelopes and 22-character Base64URL UUID v4 IDs. RCV creation accepts `order_reference` and a nonempty `items` array of `product_id`/positive-int64 `expected_quantity` pairs, without duplicate products. The document, immutable expected-quantity snapshots and creation audit commit together; actor/time come from authentication/server UTC. Duplicate SKU/ORD returns 409; invalid/missing product references return 400. An ORD is a text reference with no external synchronization.

Task 2D adds:

| Method/path | Permission | Behavior |
|---|---|---|
| POST `/api/v1/receivings/{id}/transactions` | Operator | Persist actual quantities as pending or system-rejected, both with HTTP 201 |
| GET `/api/v1/receivings/{id}/transactions` | Either role | Paginated transaction history |
| GET `/api/v1/transactions/{id}` | Either role | Transaction detail with approval/correction/review metadata |
| PATCH `/api/v1/transactions/{id}/correct` | Supervisor | Correct an unresolved rejection in place, with reason and immutable before/after audit |
| POST `/api/v1/transactions/{id}/review` | Supervisor | Resolve confirmed excess with a note; retain rejection and post no inventory |
| POST `/api/v1/transactions/{id}/approve` | Supervisor | Atomically approve, post inventory once, audit and recalculate completion |
| GET `/api/v1/inventory` | Either role | Approved inbound balances, with SKU/name search |
| GET `/api/v1/inventory/transactions` | Either role | Immutable source-linked movements, optionally filtered by `product_id` |
| GET `/api/v1/receiving-audit-logs` | Supervisor | Immutable business history, optionally filtered by `receiving_id` |

Transaction submission accepts nonempty `items` containing `receiving_document_item_id` and positive-int64 `actual_quantity`. Each line must belong to the RCV, without duplicates. Only one pending or unresolved rejected transaction is active at a time. Over-limit attempts remain `REJECTED_BY_SYSTEM`/`UNRESOLVED`; they do not alter stock. Corrections require a reason and the complete replacement item list, retain TRX/RCV IDs and retained-line IDs, and revalidate the cap. Approved and rejected/resolved transactions are terminal. Repeated approval returns 409.

Approval locks RCV then TRX and increments balances in product-ID order. Approval metadata, unique-source ledger rows, balances, completion, audit and outbox records commit together. An RCV completes only when every SKU's approved total equals its expected quantity. Inventory balances appear on first approved posting; product creation and pending/rejected transactions write no inventory. Ledger rows expose source item/TRX/RCV IDs. Identity and registration routes remain unregistered.

Task 2E adds the two locked notification operations, both available to either authenticated role:

| Method/path | Behavior |
|---|---|
| GET `/api/v1/notifications` | List the authenticated recipient's notifications using `page`/`limit`; collection `notifications` with sibling `meta` |
| PATCH `/api/v1/notifications/{id}/read` | Mark an owned notification read; repeated calls preserve its first `read_at`; inaccessible IDs return 404 |

No recipient selector, unread filter, mark-unread or extra notification route is defined. SQL predicates and services use the authenticated actor. Responses expose canonical IDs and only the current recipient's data.

The Compose `notification-worker` service shares the backend image and runs `warehouse notification-worker`. For a local binary, run the same command with `DATABASE_URL` and `RABBITMQ_URL` after migrations. It needs no JWT secret or Redis connection. It declares durable direct exchange/queue `warehouse.notifications`, with bindings for the five existing outbox event types. Messages are persistent, mandatory and publisher-confirmed. Migration `002_outbox_delivery.sql` adds delivery leases/retry bookkeeping while retaining original IDs/payloads.

Publishers claim one event through a short `FOR UPDATE SKIP LOCKED` statement and a 30-second fenced lease. Broker I/O has a five-second deadline and holds no database transaction. Transient failures release the lease with persisted backoff of 1–32 seconds; worker connection retries wait one second. Crash recovery reclaims expired leases. A confirmed publication followed by a lost database update may be delivered again; the consumer deduplicates by `(source_event_id, recipient_id)`.

Consumers use manual acknowledgement and prefetch 1. They verify the payload against its committed outbox source, then persist the complete recipient fanout atomically before acknowledgement. Database failures requeue and reconnect with delay. Duplicate delivery preserves notification ID and read state. Invalid payloads go to durable queue `warehouse.notifications.dead` for manual inspection; no automatic dead-letter replay is added. SIGTERM cancels workers and closes connections so unacknowledged deliveries can be retried. No WebSocket or external messaging is involved.

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

Tests cover T01–T08 core receiving scenarios, auth/session/RBAC, product/document validation, snapshots, correction/review, concurrent active transactions and approvals, shared-stock increments, and full rollback on inventory/outbox failures. Notification tests cover confirmations, leases/restart recovery, redelivery, database-failure acknowledgement, recipient isolation and shutdown. PostgreSQL/Redis integration tests skip explicitly unless both `TEST_DATABASE_URL` (a PostgreSQL URL) and `TEST_REDIS_URL` are set. RabbitMQ integration also requires `TEST_RABBITMQ_URL` pointing to a dedicated disposable broker/vhost because durable topology and messages are used. The suite creates and removes unique database schemas and uses isolated user/session IDs, without flushing Redis. To run them:

```sh
TEST_DATABASE_URL='postgres://user:password@127.0.0.1:5432/warehouse_auth_test?sslmode=disable' \
TEST_REDIS_URL='redis://127.0.0.1:6379/0' \
TEST_RABBITMQ_URL='amqp://user:password@127.0.0.1:5672/' \
go test -count=1 -v ./...
```

The task reports list exact executed commands, including writable temporary caches used by the agent environment. API tests authenticate through real PostgreSQL/Redis-backed login and exercise the production router. Set `CONTRACT_EVIDENCE_PATH='/tmp/warehouse-{test}.json'` to export each integration test's business responses separately for OpenAPI validation; authentication tokens are not exported. Task 2E's compiled API/worker smoke passed after correcting the temporary harness's stale credentials and discovering current test-container ports. It verified real broker outage/reconnection, duplicate delivery, recipient isolation, preserved inventory and graceful process shutdown/restart. Browser/frontend verification remains later work.

## Locked implementation conventions
Canonical 22-character Base64URL UUID v4 IDs; clone-style success/nested error envelopes and management pagination; strict binding plus validator/service checks; auth-session JWT and Redis/cookie revocation; versioned SQL migrations; typed React fetch with shared refresh and one retry. Exact contracts are in `docs/openapi.yml` and `docs/technical-spec.md`. No unresolved convention conflict remains after Task 1B.

## Frontend development

The frontend is under `frontend/`, using React/TypeScript, Vite, Tailwind, React Router and TanStack Query. Task 3A implements authentication and the responsive application shell; business pages remain clearly labeled placeholders. See `docs/task-3a-report.md` for verification and the resolved npm installation failure.

Use **Node 22.22.2 and npm 11.21.0**, the tested toolchain. `.nvmrc`, package-manager metadata and strict engine checks record the requirements. npm 9.2.0 crashed resolving the same dependency graph; npm 11 resolved it without changing library versions or bypassing peers. Node 22.22.1 was also below jsdom's declared minimum. With nvm, set up and run:

```sh
cd frontend
nvm install
nvm use
npm install --global npm@11.21.0
npm ci
# Start the backend separately; its role accounts must already be provisioned.
API_PROXY_TARGET=http://127.0.0.1:8080 npm run dev
```

Verification/build commands:

```sh
cd frontend
npm run typecheck
npm run build
npm run lint
npm test
npm run preview
```

Vite development uses localhost port 5173 and preview uses 4173. Both proxy `/api/v1` unchanged to `API_PROXY_TARGET`; `VITE_API_BASE_URL` defaults to `/api/v1`. Optional configuration is documented in `frontend/.env.example`. API requests remain same-origin because the backend uses host-only SameSite Strict cookies and has no cross-origin CORS configuration. No backend cookie or proxy configuration was changed. Production frontend container/Nginx static hosting is not implemented in this scaffold.

Routes are `/login`, `/`, `/products`, `/receivings`, `/transactions`, `/inventory`, `/notifications` and a Supervisor-only `/transactions/review` placeholder. Business areas display clearly labeled planned views without live data, metrics or notifications. The overview contains navigation/workflow guidance. No signup/reset/profile or complete business screen is added.

Authentication adapts the inspected reference's typed fetch, memory-only access token, cookie credentials, shared refresh promise and one retry to the locked nested envelopes and empty logout 204. Refresh supplies role display from JWT claims without an invented identity endpoint. Tokens are not stored in localStorage/sessionStorage. Invalid/revoked sessions clear local identity/cache; temporary unavailability hides the workspace and offers retry. If server logout cannot be confirmed, the page clears local state and explicitly offers another sign-out attempt; HttpOnly cookies require server cooperation to revoke/clear.

Focused unit/component tests run with `npm test`. Browser tests require a running frontend and an actual disposable backend with provisioned Operator/Supervisor accounts. Supply credentials and the isolated backend's test signing key through environment variables, outside the repository; the key is used only to construct a genuinely expired JWT for the test. For an installed Chrome binary:

```sh
cd frontend
E2E_BASE_URL=http://127.0.0.1:5173 \
E2E_OPERATOR_EMAIL="$OPERATOR_EMAIL" E2E_OPERATOR_PASSWORD="$OPERATOR_PASSWORD" \
E2E_SUPERVISOR_EMAIL="$SUPERVISOR_EMAIL" E2E_SUPERVISOR_PASSWORD="$SUPERVISOR_PASSWORD" \
E2E_JWT_SECRET="$JWT_SECRET_KEY" \
PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/path/to/chrome \
npm run test:browser
```

Without an executable override, Playwright uses its managed Chromium installation. Browser artifacts default to `/tmp/warehouse-playwright-results`; traces are disabled to avoid capturing authentication secrets. The full suite uses Vite development modules to exercise the actual API client. The anonymous/login and Supervisor/logout cases also ran against the production build through Vite preview. Desktop/mobile navigation, cookie rotation, concurrent expired-token refresh, backend denial/revocation and offline recovery were verified in real Chrome. No frontend business data screens or production static hosting are included yet.

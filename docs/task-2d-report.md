# Task 2D — Receiving Transactions, Review, Approval & Inventory

Date: 2026-10-22. Status: **implemented and verified for the authorized separate commit/push**. The PRD, OpenAPI and locked business contracts are unchanged. Notification delivery and frontend work remain outside this task.

## Baseline, scope decision and Git delivery

- Initial working tree was clean on `main` at Task 2C commit `dea436fde20a36c3e4d3773a2929c8c2a650a402`. `origin` was `https://github.com/Tyomaaans/Warehouse-Assesment.git`; its `main` matched that commit.
- Read the required specifications, diagrams, RBAC, testing/convention/workflow documents; inspected existing receiving repositories/models/migrations, auth, products and Task 2C before editing. Reused actual auth-session service/constructor conventions, clone-mini-erp GORM transactions/error translation and cart-style repository stubs.
- The apparent outbox scope overlap was raised before implementation. The user explicitly approved **persisting required events now, with all delivery deferred**. Existing transactional outbox inserts remain atomic with receiving changes; no publisher, consumer, dispatcher, notification route or worker was implemented.
- Task 2D is delivered separately with subject `feat: implement receiving approval and inventory workflows`. Its hash is the commit adding this report: `git log -1 --format=%H -- docs/task-2d-report.md`. The final handoff supplies the resolved hash, actual `git push origin main` result and final Git status.
- Delivery uses the explicit 24-file manifest below, staged-diff/stat/whitespace review, a separate commit and normal push. No broad add, force push, amend, reset or unrelated cleanup is used. Stop after Task 2D.

## Implemented endpoints and permissions

All paths below have `/api/v1` prefix. Existing auth, health, product and RCV endpoints remain available.

| Method/path | Permission | Behavior |
|---|---|---|
| POST `/receivings/{id}/transactions` | Operator | 201 persisted pending or system-rejected transaction |
| GET `/receivings/{id}/transactions` | Either role | Paginated transaction history, including rejected attempts |
| GET `/transactions/{id}` | Either role | Transaction detail and actor/time metadata |
| PATCH `/transactions/{id}/correct` | Supervisor | Correct unresolved rejection in place; full replacement items and nonblank reason |
| POST `/transactions/{id}/review` | Supervisor | Resolve confirmed excess with a note; remain rejected |
| POST `/transactions/{id}/approve` | Supervisor | Atomic, exactly-once approved inventory posting; repeat/concurrent losers return 409 |
| GET `/inventory` | Either role | Approved inbound balances; literal case-insensitive SKU/name search |
| GET `/inventory/transactions` | Either role | Immutable movements, optionally filtered by canonical `product_id` |
| GET `/receiving-audit-logs` | Supervisor | Immutable business audit, optionally filtered by canonical `receiving_id` |

All routes use live JWT/session/database identity checks and explicit role guards. Sensitive service methods independently require the appropriate trusted actor. Actor IDs are never accepted from requests. The 19 registered operations comprise 14 business operations plus auth/health/readiness; only the two locked notification operations remain unregistered.

## Full file inventory

Created (12 files):

- `backend/internal/receiving/transaction.go` — submission, reads, correction, resolution, approval, per-SKU revalidation and audit/outbox orchestration.
- `backend/internal/receiving/audit.go` — Supervisor audit service/read repository with consistent pagination snapshots.
- `backend/internal/receiving/transaction_test.go` — direct service role, cap revalidation, correction-policy and identity tests.
- `backend/internal/inventory/domain/inventory.go` — balance/movement read entities and repository interface.
- `backend/internal/inventory/dto.go` — exact balance/movement pagination DTOs.
- `backend/internal/inventory/repository.go` — context-bound PostgreSQL balance/ledger reads and source joins.
- `backend/internal/inventory/mapper.go` — canonical public ID mapping.
- `backend/internal/inventory/service.go` — authentication, query validation and response construction.
- `backend/internal/inventory/handler.go` — inventory query binding and shared responses.
- `backend/internal/inventory/service_test.go` — read authorization, pagination and identifier validation.
- `backend/internal/router/receiving_integration_test.go` — real PostgreSQL/Redis core workflow, concurrency, rollback, immutability and API tests.
- `docs/task-2d-report.md` — this report.

Modified (12 files):

- `backend/internal/receiving/domain/receiving.go` — audit read entity/repository contract.
- `backend/internal/receiving/dto.go` — audit response DTOs.
- `backend/internal/receiving/service.go` — receiving service interface for completed operations.
- `backend/internal/receiving/handler.go` — transaction/review/approval/history/audit handlers.
- `backend/internal/receiving/repository.go` — make transaction-history count/rows/items share a read-only repeatable-read snapshot.
- `backend/internal/receiving/service_test.go` — extend existing repository stub without removing document tests.
- `backend/internal/router/router.go` — register the nine completed operations and inject inventory handler.
- `backend/main.go` — inventory wiring.
- `backend/internal/router/workflows_integration_test.go` — shared fixture wiring, response capture across implemented business routes, per-test evidence paths and current deferred-scope assertions.
- `backend/internal/auth/auth_integration_test.go` — updated router dependencies and notification-route exclusion; existing auth checks retained.
- `README.md` — working routes, receiving semantics and verification instructions.
- `docs/ai-workflow.md` — Task 2D authorization, outbox clarification, decisions and verification evidence.

Preserved without changes: product implementation, existing receiving storage/mapper, RCV creation, row-lock/inventory-posting/audit/outbox repository methods, auth/session/token logic, middleware, shared envelopes/UUID/binding, infrastructure, Docker/Compose, dependencies and all SQL migrations. No `.env`, binary, temporary evidence or unrelated file belongs to the commit. No PRD, technical specification, OpenAPI, ERD, data-flow, RBAC, testing policy or conventions-audit contract changed.

## Business rules, atomicity and concurrency

- Submission locks the RCV first, requires `INCOMPLETE`, checks for pending or rejected/unresolved work, validates unique RCV-owned lines and positive int64 quantities, and compares each line with expected minus approved quantity. The existing partial unique index is an additional active-TRX constraint. Completed RCVs return 409.
- Any over-limit line rejects the entire attempt as `REJECTED_BY_SYSTEM`/`UNRESOLVED`, retaining actual quantities and returning 201. Both valid and rejected attempts have submission audit; rejection has its own audit event. No inventory is posted on submission, rejection, correction or resolution.
- Correction is restricted to rejected/unresolved transactions. It retains TRX/RCV IDs and retained-line IDs; removes omitted lines and creates IDs for new lines. Before/after snapshots, supplied reason, authenticated actor and server UTC time are appended immutably. A still-excessive correction stays rejected/unresolved; a valid correction becomes pending with cleared rejection metadata. Pending correction and terminal-state edits return 409.
- Review resolution retains rejected status/quantities, sets resolved note/actor/time, appends audit, and releases the active slot. It never approves excess or posts stock. Resolved rejection cannot be reopened or corrected through this API.
- Approval identifies the parent, then locks **RCV followed by TRX**, checks current pending state and recomputes per-SKU caps from approved quantities. It saves approved metadata before ledger inserts because the existing database trigger requires an approved source. These writes remain invisible until the enclosing transaction commits.
- The existing inventory posting method orders product IDs consistently. Its `INSERT ... ON CONFLICT DO UPDATE` obtains PostgreSQL balance-row/unique-key locks in that order and increments atomically, including the first posting when no balance exists. The ledger source-item unique constraint prevents duplicate movements. Concurrent approvals of different RCVs sharing products preserve all increments.
- Approval, unique-source ledger rows, balances, approval audit, completion and required outbox event are one GORM/PostgreSQL transaction. Completion uses approved totals for **every** expected SKU, never an aggregate quantity across different SKUs. Partial receipts remain incomplete; full equality sets completion time and writes before/after completion audit.
- Existing terminal-TRX/item, immutable expected-snapshot/audit/ledger and approved-source database guards remain active. Repeated/concurrent approval returns 409 without posting twice. Failures at intermediate inventory writes or the final outbox insert roll back all approval effects.
- History, audit and inventory pagination reads use read-only repeatable-read transactions so counts, rows and item/source data share a snapshot. Inventory search and filters reuse locked validation/pagination conventions. Unknown canonical filter IDs return empty pages; malformed or explicitly empty IDs return 400. Balances appear only after an approved posting. Ledger responses join immutable source item/TRX/RCV IDs.
- Every wire ID, including nested audit snapshots, uses the existing canonical 22-character Base64URL UUID v4 codec. Shared strict JSON, validator/service checks and clone-style success/errors remain unchanged. Error responses do not expose SQL, constraint names or credentials.
- Required outbox rows reuse the existing stable event-ID/recipient payload. Approval emits one approval event; submission/rejection, correction and review emit the appropriate events in their own transactions. All remain unpublished. RabbitMQ connectivity is not consulted by these transactions and notification delivery is not activated.

## Executed checks

Go 1.26.4, PostgreSQL 16 and Redis 7 were available. All required checks completed with exit code 0. Go commands ran from `/home/tyomaans/Projects/wh-assesment/backend` with:

```sh
export GOTOOLCHAIN=local
export GOPATH=/tmp/wh-go-path
export GOMODCACHE=/tmp/wh-go-mod
export GOCACHE=/tmp/wh-go-build
export TEST_DATABASE_URL='postgres://warehouse:task2d-isolated-test-password@127.0.0.1:32774/warehouse_receiving_test?sslmode=disable'
export TEST_REDIS_URL='redis://127.0.0.1:32775/0'
```

These credentials are for isolated disposable infrastructure only. Both integration suites and auth integration actually ran; no skip is counted as integration success.

| Exact command | Actual result |
|---|---|
| `gofmt -w main.go internal/receiving internal/inventory internal/router internal/auth/auth_integration_test.go` | Applied |
| `gofmt -l .` | Pass; no output |
| `CONTRACT_EVIDENCE_PATH='/tmp/wh-task2d-{test}.json' go test -count=1 -v ./...` | Pass; receiving unit 0.014s, inventory unit 0.011s; core integration 1.84s and document integration 0.81s (router package 2.676s); auth integration package 3.376s; all existing checks passed |
| `go build ./...` | Pass |
| `go vet ./...` | Pass |
| `go test -race -count=1 ./internal/receiving/... ./internal/inventory/... ./internal/router/... ./internal/auth/...` | Pass; receiving 1.080s, inventory 1.048s, router integration 18.178s, auth integration 38.650s; token/user checks also passed; no race reported |
| `go build -o /tmp/wh-task2d-api .` | Pass |
| `python3 /tmp/wh-task2d-smoke.py` | Pass; compiled-server partial/full receiving, rejection/correction, resolution/next receipt, source-linked inventory/audit, RBAC, logout invalidation and SIGTERM |
| `PYTHONPATH=/tmp/wh-contract-validation python3 -m openapi_spec_validator docs/openapi.yml` | `docs/openapi.yml: OK` |
| `PYTHONPATH=/tmp/wh-contract-validation python3 /tmp/wh-task2d-check-responses.py` | Pass; 294 recorded responses match schemas across all 14 business operations; 748 canonical UUID v4 values checked, including nested audit snapshots |
| `PYTHONPATH=/tmp/wh-contract-validation python3 /tmp/wh-check-contracts.py` | Pass; reference/operation/envelope/ID/traceability/document consistency and unchanged PRD business text; rerun after documentation completion |
| `git diff --check` | Pass; staged check/review repeated before commit |

The 11 core integration scenarios exercise T01–T08 plus full-item replacement, shared-product approvals and fault injection. They run alongside 11 existing document scenarios and the auth regression suite. Direct service tests cover permission boundaries, current-cap revalidation and the pending-correction restriction.

## Numerical and failure evidence

| Scenario | Observed result |
|---|---|
| Expected 130; submit/approve 70 then 60 | Pending stock 0; approved stock 70 with RCV incomplete/remaining 60; final stock 130, two ledger rows, completed RCV |
| Expected 10; actual 60 | Persisted 201 rejected/unresolved; stock/ledger unchanged; next submission conflicts |
| Correct 60 to 20, then 10 | First correction remains rejected; second becomes pending; same TRX/RCV and retained item ID; two immutable correction snapshots; stock increases only after approval to 10 |
| Confirm real excess, resolve, then receive 5 | Original stays rejected/resolved; no excess posting; next transaction allowed and approved stock +5, RCV incomplete |
| Eight competing submissions, pending and excessive modes | Exactly one 201 in each mode, seven 409s, one active TRX, no inventory |
| Eight competing approvals plus repeated approval | Exactly one 200, all other attempts 409; one ledger movement, balance +7, one approval outbox event |
| Two RCVs share two products; approve 3 and 4 concurrently | Both approvals succeed; each product balance is 7 with two source-linked movements; no lost increments/deadlock |
| First expected SKU fully approved, second omitted | RCV remains incomplete until second SKU also reaches its expected quantity |
| Fail the second sorted ledger insert | Earlier ledger/balance work and approved state roll back; no completion/audit/outbox side effects; retry posts once |
| Fail the final approval outbox insert | Stock, ledger, approved metadata, completion and approval/completion audit all roll back; only original submission audit/event remain; retry posts once |
| API and direct SQL attempts to mutate approved TRX/items, ledger/audit | API conflicts and existing database guards reject changes; stock/source history remain unchanged |
| Compiled-server walkthrough on one SKU | +70,+60,+10 corrected,+5 after resolution = stock 145, exactly four source-linked ledger rows; notifications remain unavailable |

Fault-injection functions/triggers exist only inside generated test schemas. Tests verify expected rejections; they are not failed verification checks. Required outbox rows remain unpublished and the notifications table remains empty.

Disposable containers were created with:

```sh
docker run -d --name wh-task2d-postgres --label warehouse-task=2d \
  -e POSTGRES_USER=warehouse -e POSTGRES_DB=warehouse_receiving_test \
  -e POSTGRES_PASSWORD=task2d-isolated-test-password \
  -p 127.0.0.1::5432 postgres:16-alpine
docker run -d --name wh-task2d-redis --label warehouse-task=2d \
  -p 127.0.0.1::6379 redis:7-alpine redis-server --save '' --appendonly no
```

Generated ports were 32774/32775. Integration tests apply/check unchanged SQL migrations in separate UUID-named schemas and remove only their own schemas/session keys. Redis is not flushed. The compiled smoke uses the disposable container's default schema and runtime-generated signing secret/account passwords. Existing project containers/data are preserved. Test containers are stopped after verification, retaining test data. Temporary response evidence, scripts, logs and binaries are under `/tmp`, outside the commit.

## Limitations and completion boundary

No unresolved contract conflict or Task 2D blocker remains. The outbox boundary was explicitly resolved by the user. Notification publishing/consumption/outbox workers and notification endpoints are deferred to Task 2E; database events intentionally accumulate unpublished. No frontend/browser or production HTTPS deployment verification was performed.

Inventory represents approved inbound receipts only. No outbound stock, adjustment, multiwarehouse, external ORD synchronization, new correction/reopening policy or unrelated feature was added. Positive int64 storage bounds still apply; database write failures roll back the operation. Integration tests require explicit PostgreSQL/Redis URLs and schema-creation privileges; missing URLs report a skip.

The backend core is ready for the authorized Task 2D commit/push and subsequent Task 2E review. No Task 2E or frontend implementation starts automatically.

# Task 2C — Product Management & Receiving Documents

Date: 2026-10-22. Status: **implemented and verified for the authorized separate commit/push**. No locked specification changed. Stop after Task 2C; receiving transactions, review/approval and inventory implementation remain for a later authorized task.

## Git baseline and delivery

- Initial `git status --short --untracked-files=all` was empty; branch was `main`.
- Task 2B was already committed as `9b91c545c23b43e7bc062eb0277a2a9ab550a375`, subject `feat: implement authentication sessions and RBAC`. Its 26-file manifest matched `docs/task-2b-report.md`, whose documented verification was confirmed. No Task 2B files needed staging or a duplicate commit.
- `origin` fetch/push URL was `https://github.com/Tyomaaans/Warehouse-Assesment.git`. `git ls-remote origin refs/heads/main` returned the same Task 2B hash before implementation. Task 2B was already pushed, and Task 2C began from a clean tree.
- Task 2C is packaged separately as `feat: implement product and receiving document workflows`, with Task 2B as its parent. The Task 2C hash is the commit adding this report, retrievable with `git log -1 --format=%H -- docs/task-2c-report.md`; the final handoff gives its resolved hash and actual push result.
- Delivery uses explicit paths from the 14-file manifest below, complete staged-diff review, staged whitespace/stat checks and `git push origin main`. No broad add, history rewrite, force push, reset or destructive cleanup is authorized.

## Implemented endpoints

| Method/path | Permission | Result |
|---|---|---|
| POST `/api/v1/products` | Supervisor only | 201 product; duplicate SKU 409 `already_exists` |
| GET `/api/v1/products` | Operator or Supervisor | Product page with optional literal, case-insensitive SKU/name search |
| POST `/api/v1/receivings` | Operator only | 201 `INCOMPLETE` document, expected SKU quantities and creation audit; duplicate ORD 409 `already_exists` |
| GET `/api/v1/receivings` | Operator or Supervisor | Summary page; optional `INCOMPLETE`/`COMPLETED` filter |
| GET `/api/v1/receivings/{id}` | Operator or Supervisor | Document detail with expected, approved and remaining quantities; malformed ID 400, missing document 404 |

Routing checks live JWT/Redis/database identity and explicit roles. Services independently require a trusted actor and the operation's role. Unauthenticated access returns 401; authenticated role denial returns 403. Frontend-supplied actor/role fields cannot grant permission and are rejected by strict request binding.

No TRX/history, correction, review, approval, inventory, notification, identity/registration or frontend endpoint was added. Existing health/readiness/auth routes remain. No RabbitMQ publishing/processing was activated. Integration tests confirm excluded routes return 404 and excluded tables receive no writes.

## File inventory

Created (6 files):

- `backend/internal/receiving/service.go` — document create/list/detail authorization, validation, entity construction and mapping.
- `backend/internal/receiving/handler.go` — strict JSON/ID/query handling and locked responses.
- `backend/internal/products/service_test.go` — direct service role, required-field, conflict and pagination guards.
- `backend/internal/receiving/service_test.go` — direct service roles, duplicate/malformed items, trusted actors, initial state and snapshots.
- `backend/internal/router/workflows_integration_test.go` — PostgreSQL/Redis-backed HTTP workflows, rollback, immutability, concurrency, contract samples and excluded-scope checks.
- `docs/task-2c-report.md` — this report.

Modified (8 files):

- `backend/internal/products/repository.go` — remove premature inventory-balance creation; retain GORM/SQLSTATE behavior.
- `backend/internal/products/service.go` — validate pagination for direct service callers before building metadata.
- `backend/pkg/request.go` — share the locked page/limit bounds between HTTP parsing and services, preventing a zero-limit division.
- `backend/internal/router/router.go` — inject both feature handlers and register exactly the five routes with role guards.
- `backend/main.go` — wire existing product and receiving repositories to their services/handlers.
- `backend/internal/auth/auth_integration_test.go` — supply the added router dependencies; update the route-exclusion assertion to the remaining unfinished workflows. All auth assertions remain.
- `README.md` — working endpoints, permissions, expected snapshots, test/response-evidence instructions and current scope.
- `docs/ai-workflow.md` — Task 2C authorization, reference reuse, decisions and executed checks.

Reused unchanged: product domain/DTO/storage/mapper/handler; receiving domain/DTO/storage/mapper/repository; authentication/session/token/middleware; shared error/UUID/actor helpers; infrastructure; SQL migrations. Correct existing files were preserved. No dependency, migration, Docker, `.env`, PRD, OpenAPI, ERD, flow, RBAC or testing-policy file changed.

## Decisions and contract compliance

- Read the requested authoritative documents and inspected actual local references before changing code. Services/constructors/early-return validation follow `/home/tyomaans/Projects/auth-session`; transactions and error translation follow the approved `/home/tyomaans/Projects/clone-mini-erp` convention. Unit tests use repository stubs and plain `testing` assertions as in `/home/tyomaans/Projects/payment-gateway/internal/payment-gateway/cart/service_test.go`.
- Product creation validates nonblank SKU/name and relies on PostgreSQL's existing unique SKU constraint. Values are preserved without inventing SKU normalization or a new uniqueness policy. Product creation writes no inventory row; inventory remains outside Task 2C.
- RCV creation validates a nonblank external ORD reference, a nonempty item list, canonical product IDs, unique product lines and positive int64 expected quantities. The existing foreign keys validate product existence inside the creation transaction; missing references become a public 400 without database details. The unique ORD constraint also handles concurrent duplicates.
- The existing repository transaction inserts the document, expected-item snapshots and `RECEIVING_CREATED` audit together. IDs are server-generated UUID v4; creator/actor derive from the verified context; timestamps are server UTC. Audit snapshots serialize public IDs canonically. Failure on a later item or the final audit insert rolls back every write.
- Expected quantities are persisted per RCV/product and protected by the existing database immutability trigger. There is no edit endpoint or external order synchronization. SKU/name display joins existing products, as specified by the locked schema; no new snapshot columns or order-management model were introduced.
- Documents start `INCOMPLETE` with null `completed_at`. Detail reuses the repository's approved-only quantity aggregate and computes remaining as expected minus approved. Task 2C tests verify the initial zero-approved state; approval/posting behavior is not claimed or activated.
- Success/errors reuse the locked clone envelopes. Every REST/document/item/actor ID uses the existing canonical 22-character Base64URL UUID v4 codec. Request binding rejects unknown fields, invalid roots/types, malformed IDs, fractional/overflowing quantities and oversized bodies.
- Pagination retains page=1/limit=10, maxima 10000/100, named collections with sibling `meta`, empty arrays and accurate totals on out-of-range pages. Status accepts only the locked enum; an explicitly empty status is invalid. Existing deterministic database ordering and escaped literal product search are reused.

## Executed verification

All required checks completed successfully. Go commands ran in `/home/tyomaans/Projects/wh-assesment/backend`, with Go 1.26.4 and these writable caches:

```sh
export GOTOOLCHAIN=local
export GOPATH=/tmp/wh-go-path
export GOMODCACHE=/tmp/wh-go-mod
export GOCACHE=/tmp/wh-go-build
export TEST_DATABASE_URL='postgres://warehouse:task2c-isolated-test-password@127.0.0.1:32772/warehouse_documents_test?sslmode=disable'
export TEST_REDIS_URL='redis://127.0.0.1:32773/0'
```

The credentials above are for disposable test infrastructure only. Integration tests actually executed; no skipped test is counted as integration coverage.

| Exact command | Actual result |
|---|---|
| `gofmt -w main.go internal/products/repository.go internal/products/service.go internal/products/service_test.go internal/receiving/service.go internal/receiving/handler.go internal/receiving/service_test.go internal/router/router.go internal/router/workflows_integration_test.go internal/auth/auth_integration_test.go pkg/request.go` | Applied; the final handler/test edit was formatted separately |
| `gofmt -l .` | Pass; no output |
| `CONTRACT_EVIDENCE_PATH=/tmp/wh-task2c-responses.json go test -count=1 -v ./...` | Pass; auth integration 3.112s, product unit 0.071s, receiving unit 0.022s, document HTTP integration 0.992s, shared contracts 0.023s; other existing tests passed |
| `go build ./...` | Pass |
| `go vet ./...` | Pass |
| `go test -race -count=1 ./internal/products/... ./internal/receiving/... ./internal/router/... ./internal/auth/...` | Pass; products 1.100s, receiving 1.065s, document HTTP integration 9.837s, auth integration 37.416s; token/user race tests also passed |
| `go test -race -count=1 ./pkg` | Pass, 1.069s |
| `go build -o /tmp/wh-task2c-api .` | Pass |
| `python3 /tmp/wh-task2c-smoke.py` | Pass; compiled-server migrations/provisioning/readiness, all five new endpoints, both-role reads, write-role denials, duplicates, snapshots, deferred routes, logout revocation and SIGTERM |
| `PYTHONPATH=/tmp/wh-contract-validation python3 -m openapi_spec_validator docs/openapi.yml` | `docs/openapi.yml: OK` |
| `PYTHONPATH=/tmp/wh-contract-validation python3 /tmp/wh-task2c-check-responses.py` | Pass; 85 actual HTTP responses conform to locked OpenAPI schemas across all five operations; 40 canonical UUID v4 values checked |
| `PYTHONPATH=/tmp/wh-contract-validation python3 /tmp/wh-check-contracts.py` | Pass; local references, 21 contract operations, envelopes, IDs, traceability, document consistency and unchanged PRD business text |
| `git diff --check` | Pass; staged diff is checked again before commit |

The 11 document-workflow integration scenarios cover empty pages/read authentication; product creation/roles/validation/duplicates; literal search and pagination; RCV creation/roles/snapshots/audit; RCV reads/status/IDs; invalid/missing/duplicate SKU lines and invalid quantities; foreign-key rollback after the first valid item; final audit-failure rollback; maximum-int64 precision; concurrent SKU/ORD uniqueness; and inactive excluded workflows. Direct service tests prove that routing is not the sole authorization boundary. Existing auth and T07 tests remain passing. Complete receiving/approval/inventory scenarios T01–T05 and the remaining T06 operations are deferred, not represented as completed coverage.

Disposable infrastructure was started with:

```sh
docker run -d --name wh-task2c-postgres --label warehouse-task=2c \
  -e POSTGRES_USER=warehouse -e POSTGRES_DB=warehouse_documents_test \
  -e POSTGRES_PASSWORD=task2c-isolated-test-password \
  -p 127.0.0.1::5432 postgres:16-alpine
docker run -d --name wh-task2c-redis --label warehouse-task=2c \
  -p 127.0.0.1::6379 redis:7-alpine redis-server --save '' --appendonly no
```

Generated host ports were 32772 and 32773. Tests apply and verify unchanged numbered/checksummed migrations in independent UUID-named PostgreSQL schemas, then remove their own schemas and Redis session keys without flushing Redis. The native smoke uses the same disposable container's default schema and runtime-generated account passwords/signing secret. Existing project containers and data are preserved. Temporary evidence, validators, smoke code, logs and binaries stay under `/tmp` and are not staged. Test containers are stopped after verification, retaining their test data.

## Limitations and handoff

No unresolved contract conflict or Task 2C blocker remains. External ORD references have no integration or synchronization. Expected snapshots cannot be edited. Product/RCV uniqueness follows the locked PostgreSQL constraints; it is not a new idempotency-key API.

The repository still contains preserved partial later-workflow methods; Task 2C does not expose or complete them. TRX history/creation/correction, review/approval, inventory posting and notifications remain inactive. Browser/frontend and production HTTPS/deployment verification remain outside this task. Integration tests require both test URLs and PostgreSQL schema-creation privileges; unset URLs produce an explicit skip.

Task 2C is ready for its verified separate commit/push and the next Task 2D review. No Task 2D implementation is started automatically.

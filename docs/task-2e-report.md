# Task 2E — Notification Outbox & Delivery

Date: 2026-10-23. Status: **implementation verified; authorized separate commit/push after final Git review**. Task 2E-Fix completed the blocked smoke without application code changes. Locked contracts and receiving/inventory rules are unchanged.

## Baseline and stopping condition

- Started with a clean working tree on `main` at `e33a244c2afe34673bddb8546bea9b7e5721097c`. Intended origin fetch/push URL: `https://github.com/Tyomaaans/Warehouse-Assesment.git`.
- Read `AGENTS.md`, PRD, technical specification, OpenAPI, ERD/data flow, RBAC, testing, conventions audit, workflow and Task 2D report. Inspected source tables, outbox writes, existing infrastructure/architecture and actual auth-session, clone-mini-erp and email-job reference code before edits.
- The user authorized commit `feat: implement notification outbox and delivery` and `git push origin main` only after every required check passed, and explicitly required stopping without delivery after failed verification.
- `python3 /tmp/wh-task2e-smoke.py` exited 1 at its first compiled `migrate` setup command. The temporary harness targeted the new Task 2E PostgreSQL port/database but retained the old Task 2D test password. The separately executed migration-upgrade harness used the correct credentials and passed. No API/worker process was started by the failed smoke command; its later scenarios did not run.
- Stopped application work and Git delivery after diagnosing that harness configuration error. Did not silently change the harness and retry delivery. Completed this evidence/handoff documentation, preserving all implementation and isolated test data. The smoke harness credential must be corrected and the required compiled API/worker verification completed on resumption.
- The user then explicitly authorized Task 2E-Fix to resume verification and delivery, preserving all 21 files. Confirmed the same baseline/branch and actual remote `main` before changes. Corrected the `/tmp` harness password, discovered reassigned Docker ports and verified credentials/labels against the existing isolated containers before compiled migration/provisioning or API/worker startup. No production/application configuration changed. The smoke passed, and all earlier implementation, test, SQL and Compose file hashes remain unchanged.

## Implemented operations

| Method/path | Permission and behavior |
|---|---|
| GET `/api/v1/notifications` | Either authenticated role; SQL-scoped to current recipient; `page`/`limit`, deterministic created-at DESC/ID ASC ordering, named `notifications` collection and locked metadata |
| PATCH `/api/v1/notifications/{id}/read` | Either role, own notification only; inaccessible IDs return 404; repeated reads preserve first `read_at` |

No other notification routes, recipient selector, unread filtering or mark-unread operation were added. Live Redis session/database-role middleware and route guards remain authoritative. Handlers and services require the trusted actor. SQL updates include both notification ID and actor ID. Responses serialize all UUIDs with the shared canonical 22-character UUID v4 codec; shared error/success envelopes and pagination are reused.

## Worker and reliability decisions

- Separate `notification-worker` binary command and Compose service, sharing the backend image. Needs PostgreSQL and RabbitMQ, with no Redis/JWT dependency. Checks migration versions before running. Publisher and consumer have independent connections and reconnect loops. The API remains independent of delivery.
- Durable direct exchange and queue `warehouse.notifications`, bound only to existing `TRANSACTION_SUBMITTED`, `TRANSACTION_REJECTED`, `TRANSACTION_CORRECTED`, `REVIEW_RESOLVED`, `TRANSACTION_APPROVED` events. Preserves existing outbox payload bytes and event IDs; AMQP message ID is the canonical outbox ID. Persistent messages, mandatory routing and publisher confirmations are enabled.
- Migration `002_outbox_delivery.sql` adds nullable fenced `lease_token`/`lease_until`, persisted attempt count and next-attempt time, plus checks/index. Original migration `001_receiving.sql` is unchanged. Existing event IDs, payloads, timestamps and publication state are retained; the upgrade was exercised against an existing pending event.
- One short autocommitted claim uses `FOR UPDATE SKIP LOCKED`, a fresh UUID lease token and a 30-second lease. No transaction or row lock remains open during broker I/O. Broker publication/confirmation has a five-second deadline; connection closure bounds socket writes, whose AMQP context alone does not cancel I/O.
- Only confirmed, routable publication permits the token-scoped published update. Failed publication releases the lease with persisted exponential delay of 1–32 seconds. Connection loops wait one second; idle polling waits 250 ms. Crash or failed database release recovers by lease expiration. Stale lease owners cannot overwrite a newer claim.
- Delivery is at least once. A crash after confirmation but before the published update can republish the same stable ID. Consumer fanout inserts in a single PostgreSQL transaction using `ON CONFLICT(source_event_id,recipient_id) DO NOTHING`; logical notifications are unique. Existing IDs/read state survive duplicates.
- Consumer checks canonical IDs, known event type and AMQP identity, then verifies the exact payload against its committed outbox source before using its recipients/message. No request or arbitrary broker payload chooses another user's notifications.
- Manual acknowledgement occurs after the whole fanout commits or duplicates are confirmed. Database failures requeue and cause reconnect/backoff; a failure on the second recipient rolls back the first. Connection loss or shutdown leaves unacknowledged messages eligible for redelivery.
- Invalid/untrusted messages are negatively acknowledged without requeue into durable `warehouse.notifications.dead`. Manual inspection is required; no replay UI, new business event type or automated dead-letter workflow was added. Structured logs contain event names/IDs rather than credentials or payload/recipient lists.
- Context cancellation closes broker connections, bounds database calls, joins both goroutines and logs shutdown. Integration tests exercised graceful worker cancellation; Task 2E-Fix also verified compiled API/worker SIGTERM exit 0 and worker restart recovery.
- No receiving service/repository, inventory posting, event generation or approval policy was changed. Task 2D's concurrency and fault-injection tests reran successfully. No external messaging or frontend implementation was added.

## Complete file inventory

Created (13):

1. `backend/internal/notifications/domain/notification.go` — entities, existing event payload and repository interfaces.
2. `backend/internal/notifications/dto.go` — exact notification and page responses.
3. `backend/internal/notifications/mapper.go` — public canonical ID conversion.
4. `backend/internal/notifications/service.go` — actor-scoped list/read and event validation.
5. `backend/internal/notifications/handler.go` — the two REST handlers.
6. `backend/internal/notifications/repository.go` — scoped SQL, consistent pagination, atomic verified fanout.
7. `backend/internal/notifications/outbox.go` — claim, fenced confirmation update and delayed retry.
8. `backend/internal/notifications/broker.go` — durable topology, mandatory confirmed publication, manual consumer.
9. `backend/internal/notifications/worker.go` — publisher/consumer loops, backoff, acknowledgement and shutdown.
10. `backend/internal/notifications/worker_test.go` — focused confirmation, acknowledgement and authorization tests.
11. `backend/internal/router/notifications_integration_test.go` — real PostgreSQL/Redis/RabbitMQ reliability/API tests.
12. `backend/migrations/002_outbox_delivery.sql` — data-preserving delivery metadata/indexes.
13. `docs/task-2e-report.md` — this evidence/handoff report.

Modified (8):

1. `backend/internal/infrastructure/rabbitmq/rabbitmq.go` — expose the existing client channel, following the reference.
2. `backend/internal/router/router.go` — notification dependency and the two guarded routes.
3. `backend/main.go` — REST wiring and separate worker command.
4. `backend/internal/router/workflows_integration_test.go` — fixture dependency and current route-exclusion assertions.
5. `backend/internal/auth/auth_integration_test.go` — fixture dependency; retain exclusion for undefined notification detail routes.
6. `compose.yml` — separate worker service, migration/broker dependencies, restart/shutdown configuration.
7. `README.md` — implemented behavior, operation and verification evidence.
8. `docs/ai-workflow.md` — authorization, design/evidence and fail-stop record.

No dependencies, `.env`, secrets, binaries or test artifacts belong to this change. No locked PRD, technical specification, OpenAPI, ERD, data flow, RBAC, testing policy or conventions audit changed. Temporary harnesses/evidence/build output live only under `/tmp`.

## Exact verification commands and results

Go commands ran from `/home/tyomaans/Projects/wh-assesment/backend` with this environment:

```sh
export GOTOOLCHAIN=local
export GOPATH=/tmp/wh-go-path
export GOMODCACHE=/tmp/wh-go-mod
export GOCACHE=/tmp/wh-go-build
export TEST_DATABASE_URL='postgres://warehouse:<redacted-test-password>@127.0.0.1:32776/warehouse_notifications_test?sslmode=disable'
export TEST_REDIS_URL='redis://127.0.0.1:32777/0'
export TEST_RABBITMQ_URL='amqp://warehouse:<redacted-test-password>@127.0.0.1:32778/'
```

Actual disposable credentials are redacted here and retained only in the temporary harness/container configuration outside the repository. The ports above record the original execution; current mappings appear below. PostgreSQL 16, Redis 7 and RabbitMQ 4 actually ran. No integration skip is counted as success.

| Command | Actual result |
|---|---|
| `gofmt -w main.go internal/notifications internal/router internal/auth/auth_integration_test.go internal/infrastructure/rabbitmq/rabbitmq.go` | Applied |
| `gofmt -l .` | Pass; no output |
| `CONTRACT_EVIDENCE_PATH='/tmp/wh-task2e-{test}.json' go test -count=1 -v ./...` | Pass; notification integration 1.77s, core integration 2.12s, document integration 1.14s; router package 5.051s, auth package 3.110s; all existing/unit checks passed |
| `go build ./...` | Pass |
| `go vet ./...` | Pass |
| `go test -race -count=1 ./internal/notifications/... ./internal/router/... ./internal/receiving/... ./internal/infrastructure/rabbitmq/...` | Pass; notifications 1.082s, router with actual three-service integration 19.811s, receiving 1.044s; no races; infrastructure has no standalone tests |
| `go build -o /tmp/wh-task2e-api .` | Pass |
| `python3 /tmp/wh-task2e-migration.py` | Pass; initialize schema at migration 001 with a real pending outbox record, run compiled migration twice, compare all original event columns byte-for-byte, verify both checksums/initialized delivery metadata |
| `PYTHONPATH=/tmp/wh-contract-validation python3 -m openapi_spec_validator docs/openapi.yml` | `docs/openapi.yml: OK` |
| `PYTHONPATH=/tmp/wh-contract-validation python3 /tmp/wh-task2e-check-responses.py` | Pass; 331 actual responses across all 16 business operations; 879 canonical UUID v4 IDs, including notification and nested audit IDs |
| `git diff --check` | Pass before smoke; repeated for final evidence documentation |
| `PYTHONPATH=/tmp/wh-contract-validation python3 /tmp/wh-check-contracts.py` | Pass after handoff documentation; 342 cross-document assertions, 21 locked operations, unchanged PRD business text |
| Initial `python3 /tmp/wh-task2e-smoke.py` | **Failed, exit 1:** `AssertionError: migrate failed`; stale Task 2D password in temporary Task 2E harness. No compiled API/worker smoke scenarios executed in that initial attempt; resolved during explicitly authorized Task 2E-Fix |

The earlier failure stopped delivery as instructed. Task 2E-Fix corrected only the temporary harness and completed the required smoke; the following final evidence supersedes that incomplete status. No application defect or code fix was required.

## Task 2E-Fix final verification

Resumption preserved the exact 21-file manifest. SHA-256 comparison against the resumption snapshot confirms all 18 non-documentation files (application, tests, migration and Compose) are unchanged. Only the already-listed README/workflow/report received final verification updates in the repository. Temporary scripts, runtime credentials, binaries and logs remain outside the repository.

Restarting the same three isolated containers retained their data but Docker reassigned host ports to **32780 PostgreSQL, 32781 Redis, 32782 AMQP, 32783 management**. The corrected smoke discovers mappings through `docker inspect`, requires the `warehouse-task=2e` label and localhost binding, verifies configured test account/database credentials against container settings and runs compiled `migrate`/`provision-users` successfully before process startup. It neither changes production credentials nor deletes existing data.

Commands executed during resumption:

| Exact command | Actual result |
|---|---|
| `docker start wh-task2e-postgres wh-task2e-redis wh-task2e-rabbitmq` | Pass; existing containers/data retained |
| `docker port wh-task2e-postgres`, `docker port wh-task2e-redis`, `docker port wh-task2e-rabbitmq` | Verified current localhost ports listed above |
| `docker exec wh-task2e-postgres pg_isready -U warehouse -d warehouse_notifications_test` | Accepting connections |
| `docker exec wh-task2e-rabbitmq rabbitmq-diagnostics -q check_running` | Fully booted/running |
| `GOTOOLCHAIN=local GOPATH=/tmp/wh-go-path GOMODCACHE=/tmp/wh-go-mod GOCACHE=/tmp/wh-go-build go build -o /tmp/wh-task2e-api .` | Pass, from `backend` |
| `python3 /tmp/wh-task2e-smoke.py` | **Pass, exit 0**; all compiled API/worker scenarios below ran |
| `GOTOOLCHAIN=local GOPATH=/tmp/wh-go-path GOMODCACHE=/tmp/wh-go-mod GOCACHE=/tmp/wh-go-build gofmt -l .` | Pass; no output, from `backend` |
| `PYTHONPATH=/tmp/wh-contract-validation python3 -m openapi_spec_validator docs/openapi.yml` | `docs/openapi.yml: OK` |
| `PYTHONPATH=/tmp/wh-contract-validation python3 /tmp/wh-task2e-check-responses.py` | Pass; revalidated the same 331 actual responses across 16 business operations and 879 canonical UUID v4 IDs |
| `env JWT_SECRET_KEY=task2e-compose-validation-key-at-least-32-bytes POSTGRES_PASSWORD=task2e-compose-validation-password RABBITMQ_PASSWORD=task2e-compose-validation-password docker compose config --quiet` | Pass; no output, placeholder validation values only |

The earlier **full Go tests, build/vet, real PostgreSQL/Redis/RabbitMQ integration tests, affected-package race tests and migration-upgrade check remain valid and are reused**, as explicitly permitted by Task 2E-Fix. No underlying implementation/test/schema/configuration file changed. The migration-upgrade harness was not rerun against existing initialized data; the smoke's compiled migration/checksum command instead confirmed that schema remains current. No skipped test is counted as integration coverage. Compose configuration passed; a full Compose image build/startup was not rerun and is not claimed.

Compiled smoke results:

- Compiled API startup, `/health`, `/ready`, both role logins, anonymous notification 401, backend role denial and logout revocation passed.
- Partial/full receiving, persisted rejected attempts, correction/resolution and exactly-once inventory behavior remained correct. Before worker startup, PostgreSQL contained exactly one unpublished approval event for the inspected approved TRX, and the current Operator had zero notifications.
- The compiled worker published/consumed the backlog through real RabbitMQ and PostgreSQL, producing **11 Operator and 7 Supervisor** notifications. GET and PATCH succeeded for the owner; another recipient's PATCH returned 404. Repeated reads retained the first timestamp.
- Republished the exact committed payload/message ID through the disposable broker management API. Waited for a new `notification_persisted` log entry from the compiled worker, then confirmed identical notification IDs, read timestamps and recipient counts. No logical duplicate was created.
- Stopped the RabbitMQ application in the isolated container. A new submission and Supervisor approval still committed, inventory rose **145 to 147**, readiness remained 200 and repeated approval returned 409. Notification count stayed unchanged during the outage. Restarted RabbitMQ; the existing compiled worker reconnected and delivered retained events, reaching **13 Operator and 8 Supervisor** notifications.
- Sent SIGTERM to the worker, required exit 0, restarted it and compared full notification snapshots. IDs/read state and counts were unchanged. Final inventory was **147 with five source-linked ledger rows**, unaffected by duplicate delivery/restart.
- Final worker and API SIGTERM both completed with exit 0. Broker operations affected only the task's isolated RabbitMQ container; existing project data was not deleted or rewritten.

## Actual reliability evidence

Seven focused RabbitMQ integration scenarios ran:

1. Eight concurrent claimants obtained exactly one owner for one pending event. An independent `FOR UPDATE NOWAIT` succeeded after claim completion, demonstrating the SQL lock was released. Expiring the lease simulated a crashed worker; a new token reclaimed the same event and the stale token was rejected.
2. Removing the real queue binding produced a mandatory broker return. Publication stayed null, lease was released and future retry time persisted. Rebinding/retrying received confirmation and marked the same source event published.
3. A PostgreSQL trigger failed insertion for the second recipient. Fanout rolled back to zero notifications; the real broker redelivered with the same message ID and `Redelivered=true`. Removing the fault permitted two committed notifications.
4. Own-recipient list/read, cross-recipient 404, repeated first-read timestamp, pagination/out-of-range metadata, anonymous 401, invalid canonical IDs and pagination errors passed through the production router.
5. Three duplicate broker publications left exactly two original notification rows and preserved their read state.
6. Closing a consumer after its database commit but before acknowledgement caused real redelivery on reconnection; duplicate processing retained two logical notifications.
7. Two concurrent worker instances drained remaining events to exactly 11 notifications: six for Operator, five for Supervisor. No `(event,recipient)` duplicate existed. Inventory remained quantity 5 with one ledger row; both workers stopped within the five-second test bound.

Task 2D's 11 core scenarios reran, including eight-way active-transaction/approval races, cross-RCV stock increments and inventory/late-outbox fault rollback. The 11 document scenarios and auth regression suite also ran. These integration results preceded the compiled smoke, which was subsequently completed during Task 2E-Fix.

## Infrastructure, remaining work and Git status

Isolated containers: `wh-task2e-postgres`, `wh-task2e-redis`, `wh-task2e-rabbitmq`, labeled `warehouse-task=2e`. Generated host ports were 32776, 32777, 32778 (AMQP), 32779 (management). Existing containers/data were untouched. Integration schemas/session keys are scoped; Redis was not flushed. The migration upgrade left its test-only original event in the disposable database. Containers are stopped at handoff, retaining data.

No verification blocker or unresolved contract conflict remains. The original harness failure is resolved with no application code fix. Full-inventory/staged-diff/whitespace review gates the authorized commit/push. Frontend/browser/production HTTPS verification is outside Task 2E.

Known operational limits: at-least-once broker transport, lease-expiry recovery delay up to 30 seconds after a crash, retries capped at 32-second intervals without an attempt cutoff, single-broker persistent storage rather than HA, manual dead-letter inspection, and no worker HTTP health endpoint (Compose supervises process/restarts; API readiness remains PG/Redis based). No event retention/deletion policy or new messaging business feature was added.

**Git delivery:** Task 2E is delivered in the separate commit `feat: implement notification outbox and delivery`, containing the reviewed 21-file manifest. Resolve its exact hash with `git log -1 --format=%H -- docs/task-2e-report.md`; the final handoff records the actual hash, normal `git push origin main` result and final working-tree/remote status. No broad add, force push, amend, reset or destructive cleanup is used. Stop after Task 2E; no frontend implementation.

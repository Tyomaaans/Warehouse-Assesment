# Combined Task 3D/3E — Frontend completion and production delivery

Date: 2026-10-23. Local mandatory gate: **passed**. Git/CI/VPS delivery: pending the authorized staged review/push at this report's initial commit; actual delivery evidence will be appended after observing execution. **Public acceptance remains pending by the owner's explicit decision. Task 4 has not been executed.**

## Baseline and scope

Started clean on main at `89105918c9abf7bd12ef000a7a644a879e3c50c3`, matching origin/main at `https://github.com/Tyomaaans/Warehouse-Assesment.git`. Read AGENTS, README, PRD, technical specification, OpenAPI, ERD/flow, RBAC, testing/conventions/workflow and Task 2D/2E/3A/3B/3C reports. Inspected actual backend inventory/notification contracts, auth/receiving/worker lifecycle and existing frontend/reference typed client before edits. All locked business/API/schema documents and dependencies/lockfiles are unchanged.

Both roles now have actual inventory balances/history, own notifications/read actions and dashboard summaries. Existing products/documents/actual receiving/correction/review/approval flows remain in place. No stock adjustments, outbound workflows, notification transport alternatives, analytics endpoint, audit viewer or unrelated feature was added. Task 4 is a plan only.

## Frontend decisions

- `/inventory`: server search/page/limit, actual SKU/name/balance/update time, explicit approved-inbound semantics and refresh. History uses `view=history` with the contract's optional canonical `product_id`; links source TRX/RCV and resolves SKU labels from the actual page's source documents. Unavailable labels retain true product IDs and source links. Pending/rejected transactions are never presented as posted stock.
- `quantity` and `quantity_delta` join the existing BigInt/raw numeric JSON codec. Unit tests and actual browser approval/stock/ledger demonstrate the entire positive int64 range, including `9223372036854775807`. Inventory uses no Number arithmetic. Existing approval invalidation reaches inventory/dashboard query prefixes; no optimistic stock or completion.
- `/notifications`: current-recipient API only, actual message/time/read state, individual mark-read with in-flight lock and server-confirmed refresh, pagination and explicit error/retry. REST polling is once per minute only for mounted visible views; no hidden-tab polling. Actor-scoped keys and existing auth cleanup isolate logout/account changes. There is no invented unread filter, mark-unread or global unread total.
- `/`: counts use server `meta.total_data` for incomplete/completed RCVs, SKUs with balances and own delivered notifications. Inventory/recent-notification lists explicitly label their five-row scope. Each query has independent loading/error/retry, so one service failure does not fabricate/erase other successful sections. No summed paginated quantity metric.
- Existing role navigation, strict payloads/envelopes/canonical IDs and accessible inputs/tables/native approval/mobile navigation were preserved. Desktop/mobile real screenshots were inspected after verification. Firefox/Safari/comprehensive accessibility testing remain future work.

## Deployment audit and implementation

Inspected the actual ERP workflow, deployment script/test, Dockerfiles and production Compose. Reused its source artifact + SSH/rsync + background bounded VPS build, immutable commit/run/attempt tags, workflow concurrency, host flock, persistent storage, pre-migration dump, readiness and atomic current symlink/previous-application rollback. Its current workflow has no registry, so no registry architecture/credentials were invented.

Warehouse adapts the one frontend, API, PostgreSQL/Redis/RabbitMQ and independent outbox-publisher/notification-consumer processes. The latter are thin command wrappers around existing tested loops; no event/receiving/inventory behavior changed. A worker infrastructure health command checks schema and broker setup; runtime health plus actual persisted-event evidence establishes delivery. API logs add raw TCP peer/client address for narrow proxy verification.

Production Dockerfiles support both actual source builds and CI-precompiled runtime layers. Bases/infrastructure are pinned to digests observed in the successful local builds. Apps are nonroot, read-only, capability-dropped, with shutdown grace periods, bounded resources/logging and loopback-only frontend/API publication. Three named volumes preserve PostgreSQL, Redis AOF and RabbitMQ state; no infrastructure port is published. No automatic pruning/deletion of releases/backups/data occurs. Numbered checksum/advisory-lock migrations remain explicit and unchanged.

The owner's host-Nginx instruction supersedes the older specification's project-Nginx deployment example. Removed only obsolete `deploy/nginx.conf`/Compose service. The frontend production application uses Node HTTP static serving, SPA fallback, cache/security headers and a same-origin local-test API relay that strips supplied forwarding headers. No project Nginx image, Dockerfile, service or configuration was added. Host Nginx was never changed/reloaded/restarted. Required operator-owned direct API routing is documented in `docs/deployment.md`.

Read-only VPS audit verified existing Warehouse SSH key access to owner-confirmed `tyomaaans@202.10.40.123:2222`, unrelated web-profile services/ports/volumes, available target path/resources, host TLS and current vhost. Initial free disk was3.2GiB; available RAM468MiB plus swap. Runtime limits total560MiB; this is not a capacity/load-test claim. Ports18082/18083 and subnet172.30.83.0/24 were unused. The existing `/opt/warehouse-assessment` had empty releases/shared and no private runtime file. Nothing unrelated was overwritten/deleted.

GitHub API metadata confirmed five existing secret names without retrieving values. The owner resolved the singular/plural discrepancy in favor of existing `DEPLOY_KNOWN_HOSTS`; workflow uses exactly `DEPLOY_HOST`, `DEPLOY_USER`, `DEPLOY_PORT`, `DEPLOY_KNOWN_HOSTS`, `DEPLOY_SSH_KEY`. No new/renamed secret or deployment variable. Strict known-host checking is retained. Production configuration stays outside releases/artifacts/images. After the mandatory local gate passed, created a new mode-0600 `/opt/warehouse-assessment/.env` with independently generated secrets and initial Operator/Supervisor accounts; no existing file was replaced, values were not printed/downloaded, and proxy trust remains empty. Initial account addresses are `operator@wh.tyomaaans.cloud` and `supervisor@wh.tyomaaans.cloud`; retrieve credentials through the owner’s SSH access only.

## Actual verification commands and evidence

Node22.22.2/npm11.21.0; Go1.26.4. Frontend commands ran from `frontend` with `PATH=/tmp/wh-task3a-toolchain/bin:$PATH` and `npm_config_cache=/tmp/wh-npm-cache`. Go used `GOTOOLCHAIN=local`, `GOPATH=/tmp/wh-go-path`, `GOMODCACHE=/tmp/wh-go-mod`; after a temporary quota failure, `GOCACHE=/home/tyomaans/Projects/wh-assesment/.work/go-cache` and `GOTMPDIR=/home/tyomaans/Projects/wh-assesment/.work/go-tmp`. Both workspace scratch directories are ignored and excluded from delivery.

| Command | Actual result |
|---|---|
| `npm ci --no-audit --no-fund` | Pass;240packages,9s; dependencies/lockfile unchanged |
| `npm run typecheck` / `npm run lint` | Pass, including final browser fixtures and Node serving code |
| `npm test` | Pass;46tests in5files,15.76s;9 new focused inventory/notification/dashboard cases plus37 existing |
| `npm run build` | Pass;1914modules,1.34s local; JS346.44kB/gzip104.60, CSS31.33kB/gzip7.92 |
| `gofmt -l backend` | Pass; empty output after formatting the three affected Go files |
| `docker build -t warehouse-api:task3de-local backend` | Pass; source builds actual API/worker executable; pinned-base rebuild retains runtime config digest `b4288c8ad09cbfc154ba61ba61f3dea70f7f55d41322dbfd299b120632c2f3f8` |
| `docker build -t warehouse-frontend:task3de-local frontend` | Pass; actual npm11.21 ci/typecheck/lint/build in image; pinned-base rebuild retains runtime config digest `dc977b4199b6a7f999db2a6aafaaba40a4d0d1c2eecf284a085c3b3fbd4936ca` |
| `python3 scripts/check-deployment.py` | Pass; exact services, loopback isolation, private envs, health/restart/persistence, existing SSH names, locking/backup/rollback and no project Nginx |
| `python3 scripts/test-deploy.py` | Pass;7 simulated activation/build/pull/migration/readiness/first-release/rollback tests,6.603s; not claimed as real VPS failures |
| `VERIFY_SKIP_BUILD=1 python3 scripts/verify-production.py` with the environment above | Pass; reused exactly the just-built images, no test skipped; full real seven-service proof below |
| `go test -count=1 -v ./...` inside gate | Pass against actual PostgreSQL16/Redis7/RabbitMQ4; no integration skip; auth,29 notification/core/document subscenarios and all units |
| `go build ./...` / `go vet ./...` inside gate | Pass |
| `go test -race -count=1 ./internal/notifications/... ./internal/router/... ./internal/receiving/...` inside gate | Pass;notifications1.045s,router20.010s,receiving1.042s with real infrastructure |
| `npm run test:browser -- tests/catalogue-receiving.spec.ts tests/receiving-transactions.spec.ts tests/inventory-notifications.spec.ts` | Pass;10actual Chrome cases against production containers,34.3s; no skips |
| `npm run test:browser -- tests/auth-shell.spec.ts --grep 'anonymous protection\|Operator login\|Supervisor navigation'` | Pass;3actual production cases,7.3s; no Vite-module introspection claimed |
| `PYTHONPATH=/tmp/wh-contract-validation python3 -m openapi_spec_validator docs/openapi.yml` | Pass; locked OpenAPI unchanged |
| `PYTHONPATH=/tmp/wh-contract-validation python3 scripts/check-responses.py /tmp/warehouse-production-cr_3cixc` | Pass;120real responses,24payloads,563canonical UUID v4 occurrences,18operations; exact int64 incl real max balance/ledger |
| `PYTHONPATH=/tmp/wh-contract-validation python3 /tmp/wh-check-contracts.py` | Pass;354cross-document/reference/contract assertions,21operations; PRD business text unchanged |
| Workflow YAML / Bash run-step syntax | Pass; all 15 run steps parse |
| `git diff --check` / `git diff --cached --check` | Pass; complete37-file staged review and secret/artifact/locked-contract inventory checks passed |

Final production evidence: `/tmp/warehouse-production-cr_3cixc`, mode0700; runtime files0600, real response/browser evidence and command logs retained. Project `wh-proof-861e3ad7`, API47265/frontend56793. Infrastructure has no published port; Go integration reaches its inspected private bridge addresses. No prior development/test volume was removed or reused. Task containers are stopped at handoff, all volumes retained. Pinned bases resolve to the exact digests inspected during successful builds. The final filter-state fix has its own rebuilt frontend image and repeated production gate evidence; earlier successful-run captures remain retained separately.

Production gate actually demonstrated:

1. Infrastructure health including no RabbitMQ local alarm; compiled migrations twice; API health/readiness; production frontend assets, deep-link SPA fallback, missing assets/private-file404; anonymous protected API401.
2. Both actual role logins and Secure/HttpOnly/Strict cookie attributes; frontend reload rotates refresh; client logout204 clears session and old access is rejected. Both-role real browser navigation/mobile state passed.
3. Product/RCV/actual receipt, pending stock0, Operator approval403, Supervisor approval2, repeated approval409, one ledger row and exactly2pending outbox events before workers.
4. Independent publisher/consumer startup, confirmed delivery to2Operator notifications, own GET/PATCH, cross-recipient404 and first read timestamp preserved.
5. Broker stopped: a further submission/approval commits, API stays ready, stock2->3 and notifications remain2. Broker recovery delivers retained events to4Operator notifications. Restart both workers; IDs/read state/counts and two ledger rows stay unchanged.
6. Recreate all application/worker containers: exact DB snapshot remains `1|2|3|2|6` (products,RCVs,total quantity,ledger rows,all recipient notifications). Data lives in the same named volumes.
7. Thirteen production Chrome cases cover actual product/document workflows, partial/full receiving, persisted rejection, same-ID correction/audit, resolution freeing a new receipt, approval race/repeated protection, every-SKU completion, both-role inventory/notifications/read state/cache account change, exact max-int64 approved balance/ledger, dashboard metadata totals and responsive/keyboard auth navigation. Browser tests use their own unique fixtures; no mock is counted as integration.
8. Measured raw direct host-to-Gin peer `172.30.98.1` while trust was empty. This is local evidence, not an assumed VPS proxy address.

## Failed iterations and corrections

- Final staged review found stale inventory search text after returning from history. Form defaults now follow the URL, with a regression assertion; final production-image and full-stack checks were repeated.
- Initial typecheck found test-only role-option/type mistakes; lint identified explicit Node globals; a mutation spy exposed TanStack's extra mutation context argument. Fixed types/imports and wrapped the API call; final46tests/typecheck/lint pass.
- Node22.22.2-alpine bundles npm10.9.7. Global replacement with npm11.21 failed at missing `promise-retry`; installing into isolated `/opt/warehouse-npm` succeeded and executed actual npm11.21 ci. No dependency constraint/peer bypass.
- A YAML flow-list comma split the unquoted tmpfs mount. Quoted it and added an absolute-mount assertion. A network-audit fixture assumed IPAM Config was iterable for host/none networks; handled null and select an unused isolated subnet.
- Initial Go compilation hit `/tmp` disk quota, not a business-test success. Preserved evidence/cache and moved build scratch to ignored disk-backed workspace directories; reran the entire gate.
- A real RabbitMQ confirmation test then failed: startup log measured105041920bytes used against a64MiB watermark, raising a publishing-blocked memory alarm. Raised broker limit to192MiB and watermark128MiB, and changed readiness to check local alarms as well as running state. The full notification integration and full production outage/recovery gate then passed. Observed broker working set was about84MiB at idle; no load-capacity claim.

Failed/aborted runs, simulated rollback cases and prior-phase checks are not counted as actual production integration passes. No business rule, endpoint, schema or application credential was weakened to make a test pass.

## Public acceptance and remaining limits

The owner explicitly selected **Leave public acceptance pending**. The existing host vhost lacks a dedicated `/api/v1/` ->127.0.0.1:18083 location; required operator configuration is in `docs/deployment.md`. No automatic Nginx change. `TRUSTED_PROXIES` remains empty until an actual request-ID-correlated public host-Nginx TCP peer can be verified. No broad/default gateway trust is introduced. Workflow explicitly records operator-pending public acceptance rather than claiming HTTPS acceptance from loopback health or frontend relay responses.

CI/VPS execution results will be appended only after they actually run. Task4's public full E2E/browser matrix/recovery drills remain unexecuted. Single-broker/light-traffic resource budgets, at-least-once transport/lease-delay recovery, modern native JSON support for high int64, per-RCV review navigation, comprehensive accessibility/browser coverage and off-host backup/restore testing remain documented limits.

## Complete Task 3D/3E file inventory

Modified/deleted tracked files:

- `.env.example`
- `.gitignore`
- `README.md`
- `backend/Dockerfile`
- `backend/internal/middleware/http.go`
- `backend/internal/notifications/worker.go`
- `backend/main.go`
- `compose.yml`
- `deploy/nginx.conf` — obsolete project Nginx configuration removed
- `docs/ai-workflow.md`
- `frontend/src/app/App.tsx`
- `frontend/src/features/overview/OverviewPage.tsx`
- `frontend/src/lib/api/json.ts`
- `frontend/src/styles.css`
- `frontend/tests/auth-shell.spec.ts`
- `frontend/tests/receiving-transactions.spec.ts`

Created files:

- `.env.production.example`
- `.github/workflows/deploy.yml`
- `compose.production.yml`
- `docs/deployment.md`
- `docs/task-3d-3e-report.md`
- `docs/task-4-test-plan.md`
- `frontend/.dockerignore`
- `frontend/Dockerfile`
- `frontend/server.mjs`
- `frontend/src/features/inventory/InventoryPage.tsx`
- `frontend/src/features/inventory/api.ts`
- `frontend/src/features/inventory/inventory-notifications.test.tsx`
- `frontend/src/features/notifications/NotificationsPage.tsx`
- `frontend/src/features/notifications/api.ts`
- `frontend/src/features/overview/WarehouseSummary.tsx`
- `frontend/tests/inventory-notifications.spec.ts`
- `scripts/check-deployment.py`
- `scripts/check-responses.py`
- `scripts/deploy.sh`
- `scripts/test-deploy.py`
- `scripts/verify-production.py`

Total: **37 files**. Excludes secrets, installed dependencies, compiled assets/binaries, runtime logs, test fixtures and ignored build caches.

## Git delivery

Implementation commit subject: `feat: complete frontend and production deployment`. Explicit file staging, complete diff/secret review and `git diff --cached --check` gate the authorized push to origin/main. Resolve the implementation hash with `git log --format=%H --grep="^feat: complete frontend and production deployment$" -1`. A documentation-only follow-up may record observed CI/VPS results without redeploying application containers. Final handoff reports actual hashes, workflow/push results and Git status. No broad add, force push, amend, reset or destructive cleanup. Stop after combined Task3D/3E.

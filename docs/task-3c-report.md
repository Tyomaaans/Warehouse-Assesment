# Task 3C — Receiving Transactions & Supervisor Decisions

Date: 2026-10-23. Status: **required verification and explicit 22-file staged review passed; ready for authorized delivery**. Scope: physical receiving entry, transaction history/detail, correction, confirmed-excess resolution and approval. No Task 3D work is included.

## Baseline and sources

- Started with clean `main` at `56f50b1e64ea3e17e54eff8b63be912001519566`, matching remote main. Origin fetch/push: `https://github.com/Tyomaaans/Warehouse-Assesment.git`.
- Read AGENTS, PRD, technical specification, OpenAPI, RBAC, testing/conventions/workflow documents and Task 2D/3A/3B reports. Inspected actual receiving handlers/DTOs/service state transitions, backend role guards and existing frontend client/query/router/components before editing.
- Rechecked `/home/tyomaans/Projects/web-profile/web-profile-frontend/src/projects/auth-session/utils/api.ts`. Preserved the feature-first React/TS organization, memory token, cookie credentials, shared refresh/single retry, nested envelopes, canonical public IDs and existing query-key conventions. No dependency, backend, migration, infrastructure configuration or locked-contract change.

## Routes, permissions and contracts

| Frontend route | Behavior |
|---|---|
| `/receivings/:id` | Both roles: expected/approved/remaining context and paginated transaction history |
| `/receivings/:id/transactions/new` | Operator only: record actual physical SKU quantities |
| `/transactions` | Both roles: choose an RCV, then open its history |
| `/transactions/:id` | Both roles: actual transaction data; Supervisor decisions when eligible |
| `/transactions/review` | Supervisor only: choose the document whose transactions need review |

The locked API scopes transaction lists to an RCV; no global transaction list/filter endpoint is invented. The review navigation uses document selection and actual per-document history, without fabricated aggregate queues or counts.

Implemented API adapters use exactly:

- `POST /api/v1/receivings/{id}/transactions` — `{items:[{receiving_document_item_id,actual_quantity}]}`; HTTP 201 for both pending and persisted rejected attempts.
- `GET /api/v1/receivings/{id}/transactions?page=&limit=` and `GET /api/v1/transactions/{id}`.
- `PATCH /api/v1/transactions/{id}/correct` — `{reason,items}` on the same ID.
- `POST /api/v1/transactions/{id}/review` — `{note}`.
- `POST /api/v1/transactions/{id}/approve` — no request body; Supervisor confirmation required.

Frontend role guards/actions use the authenticated context; sensitive action handlers also check role/eligibility. Backend RBAC remains authoritative. Request actors/timestamps/statuses are never supplied by the frontend.

## Business-rule and UI decisions

- Entry uses actual document SKU lines and shows expected, approved and remaining quantities. Unchecked SKUs are omitted; at least one positive int64 line is required. There is no remaining/expected client cap: real excess is warned about but remains submittable. BigInt and raw numeric JSON preserve `actual_quantity` exactly, including `9223372036854775807`.
- Persisted rejection is a successful creation outcome with visible `REJECTED_BY_SYSTEM`/`UNRESOLVED` feedback, saved quantities and unchanged inventory. A newest-transaction precheck provides active-work guidance; backend row locks/constraints remain the authority for races and all-history eligibility. A 409 refreshes current receiving/history state.
- Correction is offered only for rejected/unresolved transactions. Current recorded values and proposed quantities appear together; selected lines replace the submitted list. Nonblank reason is required. Both valid-to-pending and still-rejected correction outcomes are supported. Same TRX/RCV IDs and backend immutable audit behavior are preserved.
- Confirmed-excess resolution requires a nonblank note, keeps rejected status, displays resolved review metadata/note and explicitly explains that inventory is unchanged. It releases the active slot for a subsequent receipt; no override or reopening action exists.
- Approval is offered only for pending transactions to Supervisor. Native dialog confirmation supports cancel/Escape; in-flight ref locks and disabled controls prevent repeated submission. No inventory or completion is updated optimistically. Confirmed responses seed only transaction data; actor-scoped document/history/inventory caches are invalidated/refetched. Receiving completion is displayed only from the backend document response.
- Conflicts refresh live transaction/document state, preserve public backend errors and remove actions once the transaction becomes terminal. Approved transactions and resolved rejections have no correction/approval controls. Status notices also reflect the current returned state after reload.
- Detail displays only actual contract fields: submitted quantities, current receiving quantities, creation/update data, approval actor/time, latest correction actor/time, rejection reason, review note and resolution actor/time. Full immutable correction reasons/history remain in backend audit; no standalone audit viewer is added.
- Reused design tokens, shared inputs/buttons/tables/loading/error states and mobile navigation. Added a small shared native confirmation dialog. Responsive context/history tables scroll inside labeled keyboard-focusable regions. No inventory/notification screen or dashboard metric is implemented.

## Complete changed-file inventory

Created (11):

- `docs/task-3c-report.md`
- `frontend/src/components/ui/ConfirmDialog.tsx`
- `frontend/src/features/transactions/api.ts`
- `frontend/src/features/transactions/queries.ts`
- `frontend/src/features/transactions/TransactionStatus.tsx`
- `frontend/src/features/transactions/ActualQuantityForm.tsx`
- `frontend/src/features/transactions/TransactionHistory.tsx`
- `frontend/src/features/transactions/TransactionEntryPage.tsx`
- `frontend/src/features/transactions/TransactionDetailPage.tsx`
- `frontend/src/features/transactions/transactions.test.tsx`
- `frontend/tests/receiving-transactions.spec.ts`

Modified (11):

- `README.md` — working receiving/Supervisor routes and browser instructions.
- `docs/ai-workflow.md` — authorization, decisions and execution evidence.
- `frontend/src/app/App.tsx` — transaction routes and role guards.
- `frontend/src/app/workflows.test.tsx` — receiving-detail history API fixture.
- `frontend/src/components/layout/AppShell.tsx` — transaction subroute navigation/breadcrumbs.
- `frontend/src/features/receiving/ReceivingDetailPage.tsx` — transaction history integration.
- `frontend/src/features/receiving/ReceivingListPage.tsx` — document selection for transaction/review navigation.
- `frontend/src/lib/api/json.ts` — exact decoding of `actual_quantity`.
- `frontend/src/styles.css` — transaction/status/form/dialog layouts.
- `frontend/tests/auth-shell.spec.ts` — transactions are implemented; inventory/notifications remain placeholders.
- `frontend/tests/catalogue-receiving.spec.ts` — scope creation notice separately from history loading; validate returned completed documents instead of assuming an empty global filter.

Total: **22 files**. No secrets, `.env`, node_modules, dist, logs, binaries or temporary fixtures/scripts belong to the commit. Existing work is preserved.

## Verification commands and results

Npm commands ran from `frontend` with `env PATH=/tmp/wh-task3a-toolchain/bin:$PATH npm_config_cache=/tmp/wh-npm-cache`, using the documented **Node 22.22.2 / npm 11.21.0**. Go build ran from `backend`.

| Command | Actual result |
|---|---|
| `npm ci --no-audit --no-fund` | Pass; 240 packages, 10s; unchanged lockfile |
| `npm run typecheck` | Pass; final browser-test adjustment rechecked |
| `npm run lint` | Pass; final browser-test adjustment rechecked |
| `npm run build` | Pass; 1910 modules, 1.11s; JS 332.98 kB (gzip 101.79), CSS 28.21 kB (gzip 7.39) |
| `npm test` | Pass; 37 tests in 4 files, 24.27s; 13 new transaction tests plus 24 existing auth/product/document tests |
| `env GOTOOLCHAIN=local GOPATH=/tmp/wh-go-path GOMODCACHE=/tmp/wh-go-mod GOCACHE=/tmp/wh-go-build go build -o /tmp/wh-task3c-api .` | Pass; actual unchanged backend compiled |
| `python3 /tmp/wh-task3c-infra.py prepare` | Created fresh isolated infrastructure/runtime; first migration invocation failed. Retry of compiled `migrate` and `provision-users` with the unchanged generated environment passed. Initial stderr was not retained by the launcher, so its precise first failure cause is unconfirmed |
| `python3 /tmp/wh-task3c-infra.py browser` | Pass; both full browser suites, API health/readiness and SIGTERM exit 0 |
| `npm run test:browser` | Pass; all 11 actual Chrome/backend development cases, 1.3m, no skips |
| `npm run test:browser -- tests/receiving-transactions.spec.ts` | Pass; all three receiving cases repeated against production build/Vite preview, 19.9s, no skips |
| `PYTHONPATH=/tmp/wh-contract-validation python3 -m openapi_spec_validator docs/openapi.yml` | Pass; locked OpenAPI unchanged |
| `PYTHONPATH=/tmp/wh-contract-validation python3 /tmp/wh-task3c-contracts.py` | Pass; 140 real responses, 37 exact payloads, 676 canonical UUID occurrences across 14 operations, including all six transaction operations; schema/format and int64 checks pass |
| `PYTHONPATH=/tmp/wh-contract-validation python3 /tmp/wh-check-contracts.py` | Pass; 348 consistency assertions across 21 operations; PRD business text unchanged |
| `git diff --check` and `git diff --cached --check` | Pass; reviewed all 22 staged files and verified their content against the reviewed SHA256 inventory; no unrelated/unstaged/untracked files |

Initial iterations caught an unused type import (typecheck/lint), missing native dialog methods in jsdom, and an overly broad unit text selector matching both SKU context labels. Removed the unused import, provided explicit test-only dialog stubs and scoped the selector. Native dialog behavior is tested in real Chrome, not claimed from jsdom. Browser regressions then exposed old Task 3B assumptions: one status region per page and no completed documents in the test database. Updated assertions to distinguish the history loader and to check actual returned completed rows. Failed/skipped iterations are not counted as passing coverage. No backend/configuration workaround was introduced.

## Actual backend/browser evidence

Fresh labeled containers: `wh-task3c-postgres` (PostgreSQL 16, host 32790), `wh-task3c-redis` (Redis 7, 32791), `wh-task3c-rabbitmq` (RabbitMQ 4, 32792). API 51979, Vite dev 53863, preview 57609. Generated role credentials/signing key are retained only in mode-0600 `/tmp/wh-task3c-runtime.json`; tests create unique ORD/SKU fixtures. Existing project data/containers are untouched. No notification worker is required; existing backend outbox writes remain intact.

The three actual receiving scenarios cover:

1. Expected A=10/B=5. Operator records A=4 pending; no stock before approval. Active follow-up is blocked in UI and returns backend 409. Supervisor confirms approval; A stock=4/one source ledger row. Repeated approval returns 409 without another posting; document remains incomplete.
2. Operator records A=60 as persisted HTTP 201 rejected/unresolved. Supervisor corrects the same TRX/RCV to A=6 with a reason. Audit API verifies before=60, after=6, reason and Supervisor actor. Correction itself leaves stock=4; subsequent approval raises A to10/two ledger rows. B remains outstanding, so RCV remains incomplete.
3. Operator records B=8 as rejected. Supervisor verifies/resolves physical excess with a note; rejected/resolved stays rejected and B has no inventory row/movement. Operator can then record B=5; approval creates B stock=5/one source-linked movement and completes the RCV only after both SKU totals match. Approved data stays immutable; completed entry is closed. History retains all four transactions.
4. Separate expected=1 fixture accepts physical actual=`9223372036854775807`, persists rejected HTTP 201 and displays the exact quantity. Correction to `9223372036854775806` retains ID and rejected/unresolved state; neither operation posts inventory.
5. Competing Supervisor approval succeeds while a UI confirmation holds stale pending data. UI approval returns 409, refreshes to immutable approved/completed state and stock remains2 with one ledger row.
6. Real backend role denials: Operator approval/correction/review and Supervisor transaction submission return403; Supervisor direct entry route is denied. Both roles read permitted receiving/transaction data.

Stock, ledger and audit checks use actual existing backend read APIs, without adding their frontend screens. Captures retain raw response/request JSON, so Python schema/ID/int64 checks do not round large values. Screenshots under `/tmp/wh-task3c-screenshots` were visually inspected for correction context and mobile history; viewport overflow is checked in Chrome. Final evidence is isolated from failed runs under `/tmp/wh-task3c-contracts-verified`.

The final 14 browser executions comprise 11 distinct development cases and three production-preview repetitions; no mocked tests or previously skipped cases are counted as real integration passes. `docker exec wh-task3c-postgres psql -U warehouse -d warehouse_frontend_test -tAc 'SELECT count(*), count(DISTINCT receiving_transaction_item_id), sum(quantity_delta) FROM inventory_transactions;'` returned `16|16|68`. This total includes four retained test campaigns, including earlier regression failures; each final fixture independently checks its own A=10/B=5 or race=2 balance and ledger entries. All ledger sources remain unique. The compiled API exited 0 on SIGTERM; dev/preview process groups stopped. Task 3C containers are stopped at handoff with data retained.

## Limitations and Git delivery

Inventory and notification screens, production frontend hosting/HTTPS, Safari/Firefox and a comprehensive accessibility audit remain deferred. Review navigation is per-document rather than a global filtered queue because the API has no such endpoint. Full audit history has no standalone UI in this task. Concurrent changes become visible on navigation/refresh or mutation conflict; backend validation remains authoritative. High-range int64 values require the modern JSON source/raw-number support documented since Task 3B. No chat/WebSocket, outbound feature or approval override was added.

After all checks and the explicit 22-file staged review pass, commit `feat: implement receiving transaction and approval UI` and run `git push origin main`. The commit containing this report is the Task 3C delivery; resolve it with `git log -1 --format=%H -- docs/task-3c-report.md`. Final handoff reports its actual hash/push result/clean status. Never broad-add, force push, amend, reset or destructively clean existing work. Stop after Task 3C.

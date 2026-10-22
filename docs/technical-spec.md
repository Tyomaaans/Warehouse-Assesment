# Technical Specification — Warehouse Receiving System
**Version:** 1.1 | **Derived from:** `docs/prd.md` | **Conventions locked:** Agent Task 1B, 2026-10-22

## 1. Architecture
**Backend:** Go + Gin, feature-first/Screaming Architecture, GORM, PostgreSQL, zerolog, JWT (15-minute access token), Redis-backed hashed refresh tokens with session revocation, RabbitMQ notifications. **Frontend:** React + TypeScript + Vite + Tailwind CSS + React Router + TanStack Query. **Ingress:** Nginx reverse proxy, trusted Gin proxies explicitly configured. **Deployment:** Docker Compose to personal server.

A single Go application hosts feature-first modules following `/home/tyomaans/Projects/auth-session`: `internal/<feature>/domain` holds entities/repository interfaces; feature packages own `handler.go`, `service.go`, `repository.go`, `dto.go`, `storage.go`, and `mapper.go` where applicable. Service interfaces live beside their implementations; exported `NewX` constructors inject dependencies, and errors use early returns. Infrastructure, middleware, configuration and router wiring remain shared. Suggested warehouse features: `auth`, `products`, `receiving`, `inventory`, `notifications`, `audit`. Avoid generic global handler/service/repository buckets.

**Interactions:** Browser -> Nginx -> Gin REST -> PostgreSQL; auth -> Redis; business transaction -> PostgreSQL outbox -> RabbitMQ publisher -> notification consumer -> PostgreSQL notifications -> browser REST polling. No Gorilla, chat, or WebSocket. RabbitMQ does not itself deliver browser push; the baseline uses polling of persisted notifications via TanStack Query. Broker/consumer failures must not invalidate receiving commits.

**Logs:** zerolog for runtime/security logs; `receiving_audit_logs` and `inventory_transactions` for durable structured business history. The inspected references do not require a database-backed system log table. Never put passwords, raw refresh tokens, or secrets in logs.

## 2. Identifier Policy
Every table primary key is UUID v4, including migration bookkeeping. Use PostgreSQL `uuid` columns for PK/FK. Serialize every REST ID (body, path, query, cookie and audit snapshot) as canonical **unpadded Base64URL of the UUID's 16 binary bytes**, using `base64.RawURLEncoding` as in `/home/tyomaans/Projects/auth-session/internal/auth-session/user/mapper.go`. Output is exactly 22 characters. Decode input, require 16 bytes, RFC 4122 variant and version 4, then require exact re-encoding equality. Reject textual UUIDs, padding, whitespace, alternate alphabets and noncanonical pad bits. The OpenAPI pattern constrains alphabet/length/pad bits; server validation also checks UUID version/variant. JWT claims retain the reference's internal user/session UUID representation; they are signed token internals, not REST ID DTOs. ORD remains an external string. Do not add human-readable RCV/TRX numbers.

## 3. Data Model
See `docs/erd.mmd`. Recommended tables (all UUID v4 PKs):
- `users`: `id uuid`, `email varchar unique`, `password_hash text`, `role varchar` (`OPERATOR|SUPERVISOR`), `created_at timestamptz`.
- `products`: `id uuid`, `sku varchar unique`, `name varchar`, `created_at timestamptz`.
- `receiving_documents`: `id uuid`, `order_reference varchar unique`, `status varchar` (`INCOMPLETE|COMPLETED`), `created_by uuid FK users`, `created_at/updated_at timestamptz`, `completed_at timestamptz nullable`.
- `receiving_document_items`: `id uuid`, `receiving_document_id uuid FK`, `product_id uuid FK`, `expected_quantity bigint CHECK > 0`, UNIQUE(document, product).
- `receiving_transactions`: `id uuid`, `receiving_document_id uuid FK`, `status varchar` (`PENDING_APPROVAL|APPROVED|REJECTED_BY_SYSTEM`), `review_status varchar nullable` (`UNRESOLVED|RESOLVED` for rejected), `rejection_reason text nullable`, `review_note text nullable`, `created_by uuid FK`, `approved_by/corrected_by/reviewed_by uuid nullable FK users`, `approved_at/corrected_at/reviewed_at timestamptz nullable`, `created_at/updated_at timestamptz`. Actor/time pairs are both null or both set. Correction fields identify the latest correction; review fields identify resolution. All corrections retain full reason, actor, timestamp and before/after data in the append-only audit.
- `receiving_transaction_items`: `id uuid`, `receiving_transaction_id uuid FK`, `receiving_document_item_id uuid FK`, `actual_quantity bigint CHECK > 0`, UNIQUE(transaction, document_item).
- `inventory_balances`: `id uuid`, `product_id uuid FK unique`, `quantity bigint NOT NULL DEFAULT 0 CHECK >= 0`, `updated_at timestamptz`.
- `inventory_transactions`: `id uuid`, `product_id uuid FK`, `receiving_transaction_item_id uuid FK unique`, `quantity_delta bigint CHECK > 0`, `created_at timestamptz`. API responses join the source item and parent TRX to expose both `receiving_transaction_id` and `receiving_document_id`; these links remain immutable.
- `receiving_audit_logs`: `id uuid`, `receiving_transaction_id uuid nullable FK`, `receiving_document_id uuid FK`, `actor_id uuid nullable FK`, `event_type varchar`, `before_data jsonb nullable`, `after_data jsonb nullable`, `reason text nullable`, `created_at timestamptz`. Append-only.
- `notifications`: `id uuid`, `recipient_id uuid FK users`, `source_event_id uuid FK notification_outbox`, `event_type varchar`, `receiving_transaction_id uuid nullable FK`, `message text`, `read_at timestamptz nullable`, `created_at timestamptz`; **UNIQUE(source_event_id, recipient_id)** permits fan-out and deduplicates retries per recipient.
- `notification_outbox`: `id uuid`, `event_type varchar`, `aggregate_id uuid`, `payload jsonb`, `published_at timestamptz nullable`, `created_at timestamptz`; retry-safe publish.

**Important constraints/indexes:** unique SKU, unique ORD reference, unique RCV SKU line, unique TRX RCV-item line, unique inventory ledger source item, unique inventory balance product; indexes on TRX(document,status), notifications(recipient,created_at), outbox(published_at,created_at), audit(document,created_at), inventory transactions(product,created_at).

**Single active TRX constraint:** PostgreSQL partial unique index:
`CREATE UNIQUE INDEX uq_active_trx_per_rcv ON receiving_transactions(receiving_document_id) WHERE status = 'PENDING_APPROVAL' OR (status = 'REJECTED_BY_SYSTEM' AND review_status = 'UNRESOLVED');`
Keep `review_status` null for non-rejected TRXs. All TRX item `receiving_document_item_id` values must belong to the parent TRX's RCV; validate in service inside the transaction.

**Migration policy:** use numbered SQL migrations and `/home/tyomaans/Projects/clone-mini-erp/backend/migrations/migrate.go`'s embedded-file, SHA-256 checksum and PostgreSQL transaction-scoped advisory-lock pattern. Record applied filename, checksum and time in `schema_migrations` with a UUID v4 `id` PK and unique filename. Refuse modified applied migrations. Acquire the migration lock before reading/updating bookkeeping and apply atomically. Run migrations explicitly before serving. GORM handles context-bound querying/persistence and transaction callbacks; no runtime AutoMigrate. Use SQLSTATE-to-sentinel translation without exposing database details.

## 4. Transaction and Concurrency Design
**Create RCV:** transaction inserts RCV and unique expected SKU lines. Reject duplicate ORD reference, SKU lines, invalid quantities, unknown products.

**Submit TRX:** transaction locks RCV row (`SELECT ... FOR UPDATE`), checks RCV incomplete and no active TRX, validates all item IDs belong to RCV, ensures no duplicate items, records positive physical actual quantities. Calculate remaining per SKU from **APPROVED** TRX items only. If any submitted item exceeds remaining, save the entire TRX as `REJECTED_BY_SYSTEM` with `UNRESOLVED` review, preserving entered actual quantities; otherwise save `PENDING_APPROVAL`. Do not update inventory in either case. No zero-quantity line is accepted; omitted SKU means no receipt for that SKU.

**Correct rejected TRX:** SPV only; lock RCV then TRX, require rejected+unresolved, audit before/after and non-whitespace reason, replace the submitted item list on the same TRX, retain item IDs for retained RCV lines, set corrected_by/at from authenticated identity/server time, and revalidate. If valid -> `PENDING_APPROVAL`, review_status NULL and cleared rejection reason; if invalid -> rejected+unresolved with refreshed rejection reason. Record every correction in immutable audit, including repeated still-invalid corrections. Review resolution metadata remains null until resolution. No inventory posting.

**Resolve confirmed physical discrepancy:** SPV only; rejected+unresolved -> rejected+resolved with review note/actor/time; append audit. RCV remains incomplete. Subsequent TRX allowed.

**Approve:** SPV only; in one PostgreSQL transaction lock RCV then TRX, require pending, recompute remaining against approved quantities, return 409 if the cap would be exceeded, insert one inventory ledger row per TRX item (unique source item), atomically increment balances, mark TRX approved with approver/time, recalculate each RCV SKU's approved sum, mark RCV completed with completed_at only if every SKU matches expected, append audit and notification outbox. Commit once. Repeated or concurrent approval returns **409 without posting again**. All writes roll back together on failure. Parallel approval and new TRX creation serialize on RCV row; update balances in product-ID order to avoid conflicting multi-product lock orders.

**Outbox delivery:** write the stable event ID, TRX ID, event type and recipient UUID list into the transactional outbox. Submission/rejection/correction/resolution notify Supervisors and the submitting Operator as relevant; approval notifies the submitting Operator. Do not expose other recipients in notification APIs. Publisher retries unpublished rows to RabbitMQ and marks published only after broker confirmation. Consumer inserts with **ON CONFLICT(source_event_id, recipient_id) DO NOTHING**, commits before acknowledgement, and retries failures. Stable event IDs equal outbox IDs; fan-out creates a row per recipient. Broker duplicates never duplicate stock or notifications. Inventory commits never wait for broker availability. RabbitMQ worker integration belongs to Prompt 3; Prompt 2 must already write the outbox atomically.

## 5. State Transitions
TRX:
- Creation with valid quantities -> `PENDING_APPROVAL`.
- Creation with over-limit quantities -> `REJECTED_BY_SYSTEM` + `UNRESOLVED`.
- Pending -> Approved (SPV approval).
- Rejected/unresolved -> Pending (SPV correction now valid).
- Rejected/unresolved -> Rejected/unresolved (correction still invalid).
- Rejected/unresolved -> Rejected/resolved (SPV confirms physical discrepancy).
- Approved is terminal. Rejected/resolved is terminal in baseline.

RCV: `INCOMPLETE` on creation; `INCOMPLETE` -> `COMPLETED` only after approval makes every SKU exactly fulfilled. Completed is terminal. Rejected TRX does not change RCV status.

## 6. Authorization
See `docs/rbac.md`. Use the JWT/session model in `/home/tyomaans/Projects/auth-session`: HS256 access JWTs expire in **15 minutes**, include user/session/role claims and require a live Redis session on every protected request. Generate refresh tokens from 32 cryptographically random bytes encoded as hex; persist SHA-256 hashes under `refresh:<hash>`, JSON sessions under `session:<SHA256(session UUID)>`, and per-user session sets under `user_sessions:<user UUID>`. Rotate once under the reference's `SET NX` lock with a bounded TTL and ownership-checked release; return 409 on concurrent rotation. Revocation deletes session and refresh records, immediately invalidating access JWTs. Redis failure must never bypass authentication; return a generic 503. Verify signature, algorithm, expiry and claims before use.

Transport: access token in JSON and then Authorization Bearer; refresh token never in JSON. Set host-only HttpOnly **refresh_token** (Path=`/api/v1/auth`) and **session_id** (Path=`/api/v1`) cookies, SameSite=Strict, Secure in production/HTTPS. The session cookie uses the canonical Base64URL ID. Cookie Max-Age follows Redis refresh/session TTL: `SHORT_REFRESH_EXPIRY=1h` by default, `DEFAULT_REFRESH_EXPIRY=168h` for remember_me, matching the inspected reference example. Rotation replaces both cookies without changing the session ID. Logout requires bearer plus both cookies, revokes the current session, clears both cookies at identical paths/attributes, and returns **204 with no body**. Do not add public registration or session-management business features. Provision the two role accounts through operator tooling/environment configuration.

Check roles in route middleware and sensitive service operations. Notifications use recipient predicates in SQL; inaccessible notification IDs return 404. Other role denials return 403; missing/invalid/revoked authentication returns 401. Use the same nested error envelope everywhere.

## 7. API Contract
See the locked `docs/openapi.yml`. Success uses `{"success":true,"message":"optional public message","data":<payload>}`; the only empty-body success is logout 204. Errors use the complete clone envelope `{"success":false,"error":{"code":"invalid_input","message":"items[0].actual_quantity must be a positive integer"}}`. Return useful public validation/state messages; never expose SQL, constraint names, stack traces, password hashes, secrets or token values. Public error codes/statuses are specified in OpenAPI; preserve documented 400/401/403/404/409 behavior, 413 body limits, and generic 500/503 responses. Over-limit TRX submission is **201 with a rejected Transaction payload**, never a request validation error.

Pagination follows clone's authenticated management convention: `page=1` (maximum 10000), `limit=10` (maximum 100). Example: `{"success":true,"data":{"products":[],"meta":{"current_page":1,"total_pages":0,"page_size":10,"total_data":0}}}`. Other named collections are `receivings`, `transactions`, `balances`, `movements`, `audit_logs`, `notifications`; no extra nesting. Metadata is snake_case, total_pages=ceil(total_data/page_size), empty collections are [] and total_pages=0. Out-of-range pages return 200 with an empty array and accurate totals. Use deterministic created_at DESC/id ASC ordering; balances use SKU/id ordering. `search` remains the query name, with literal case-insensitive substring matching over SKU/name and at most 100 characters. No fuzzy-search or catalogue-filter API is added.

Strict request binding follows clone: one non-null JSON object, valid UTF-8, no unknown fields or trailing JSON values, maximum 1 MiB. Reject malformed Base64URL IDs before SQL. Use validator tags for structural requirements and service validation for positive int64 quantities, non-whitespace names/SKUs/order references/reasons/notes, duplicate lines, parent ownership, remaining caps, active TRX and state transitions. Actor IDs and timestamps always come from verified identity/server UTC time. API audit snapshots use the same public ID encoding. Request schemas forbid client-supplied actor/timestamp fields.

## 8. Data Flow and API Flow
See `docs/data-flow.mmd`. Primary workflow:
1. SPV creates products.
2. Operator creates RCV with expected lines.
3. Operator submits actual quantities for a TRX.
4. Server locks RCV, validates remaining and single-active policy.
5. Invalid attempt -> rejected+unresolved, audit, outbox; SPV verifies and corrects or resolves.
6. Valid attempt -> pending, audit, outbox.
7. SPV approves; transaction posts inventory ledger/balances, recalculates RCV completion, and writes audit/outbox.
8. Publisher delivers RabbitMQ event; consumer stores notification; UI polls notifications.
9. UI displays inventory balances and transaction history.

## 9. Frontend / UX
Role-aware pages: Login, Products, RCV List/Create, RCV Detail with expected/approved/remaining, TRX Submission, SPV Pending/Rejected Queue, TRX Review/Correction, Inventory, Inventory History, Notifications. No chat UI. Reuse `/home/tyomaans/Projects/web-profile/web-profile-frontend/src/projects/auth-session/utils/api.ts`'s typed fetch client, in-memory access token, cookie credentials, shared refresh promise and single retry. Adapt error extraction to `error.code/error.message` and parse 204 without JSON. UI role information after refresh can be read from JWT claims for display; backend checks remain authoritative. Use centralized environment-backed API configuration. Provide field-level local validation, server messages, loading, empty, retry, forbidden, network error and confirmation before approval. Use TanStack Query invalidation after mutations and REST polling for persisted notifications; the library integration is specification-driven because no inspected reference establishes it.

## 10. Critical Testing Strategy
See `docs/testing.md`. Focus on a small set of high-value automated unit/integration/API checks: partial -> full completion, over-limit rejection, double approval idempotency, single active TRX, correction audit, role denial, and one validation/error contract case. One short manual end-to-end smoke walkthrough is sufficient. Do not aim for broad line coverage.

## 11. Deployment
Dockerfiles for frontend/backend, Compose services for Nginx, API, PostgreSQL, Redis, RabbitMQ and notification worker (worker may share backend image), env configuration, persistent DB/broker volumes, versioned migrations, health/readiness, clean-start README. No secrets committed. Nginx forwards necessary headers; Gin trusted proxy CIDRs must match actual Docker network or known reverse-proxy addresses. Restrict RabbitMQ/Redis/Postgres ports from public ingress.

## 12. Assessment Traceability
| Assessment requirement | Evidence |
|---|---|
| PRD | `docs/prd.md` |
| Architecture, ERD, DB, API, flows, states, auth, tests | This document, `docs/erd.mmd`, `docs/openapi.yml`, `docs/data-flow.mmd`, `docs/rbac.md`, `docs/testing.md` |
| Working core app | Implementation, demo, README |
| AI-driven process and >=3 meaningful prompts | `AGENTS.md`, `docs/ai-workflow.md` |
| Critical tests and stock evidence | `docs/testing.md`, test results |
| Docker/migrations/health/setup | Dockerfiles, compose, migrations, README |
| Git history | Meaningful commits |
| Known limitations | README and testing notes |

**Assessment guardrail:** The assessment explicitly prioritizes core workflow correctness over visual polish or feature quantity. RabbitMQ/Redis/Nginx are user-selected technical infrastructure; do not extend them into unrelated business features.
